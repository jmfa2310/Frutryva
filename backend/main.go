package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"futryva/backend/internal/config"
	"futryva/backend/internal/httpapi"
	"futryva/backend/internal/identity"
	"futryva/backend/internal/migrate"
	"futryva/backend/internal/organizations"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if len(os.Args) == 2 && os.Args[1] == "healthcheck" {
		if err := healthcheck(); err != nil {
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "owner" {
		if len(os.Args) != 4 || os.Args[2] != "set" {
			logger.Error("Use: /futryva owner set EMAIL_DA_CONTA_CONFIRMADA")
			os.Exit(1)
		}
		if err := setOwner(os.Args[3]); err != nil {
			logger.Error("Nao foi possivel configurar o dono da plataforma", "reason", err.Error())
			os.Exit(1)
		}
		logger.Info("Conta configurada como dona da plataforma. Volte a abrir o Manager para ver Administracao.")
		return
	}
	if err := run(logger); err != nil {
		logger.Error("API stopped", "reason", err.Error())
		os.Exit(1)
	}
}

// setOwner is deliberately a local operator command, never a registration or
// HTTP privilege escalation path. The account must already verify its email.
func setOwner(email string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.StartupTimeout)
	defer cancel()
	db, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return errors.New("invalid PostgreSQL configuration")
	}
	defer db.Close()
	if err := migrate.Apply(ctx, db, cfg.MigrationsDir); err != nil {
		return errors.New("database migrations failed")
	}
	return organizations.SetPlatformOwner(ctx, db, email)
}

// healthcheck is used by Docker without a shell, curl or database secrets.
func healthcheck() error {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	client := &http.Client{Timeout: 3 * time.Second}
	response, err := client.Get("http://" + net.JoinHostPort("127.0.0.1", port) + "/health")
	if err != nil {
		return errors.New("local API healthcheck failed")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return errors.New("local API is not ready")
	}
	return nil
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	startup, cancel := context.WithTimeout(ctx, cfg.StartupTimeout)
	defer cancel()
	db, err := pgxpool.New(startup, cfg.DatabaseURL)
	if err != nil {
		return errors.New("invalid PostgreSQL configuration; inspect PG* variables or DATABASE_URL")
	}
	defer db.Close()
	if err := db.Ping(startup); err != nil {
		return errors.New("PostgreSQL connection failed; check database readiness, credentials and TLS configuration")
	}
	if err := migrate.Apply(startup, db, cfg.MigrationsDir); err != nil {
		return errors.New("database migrations failed; inspect the migration files and database schema before restarting")
	}
	cancel()
	mailer, err := identity.NewSMTPMailer(cfg.SMTPHost, cfg.SMTPPort)
	if err != nil {
		return errors.New("invalid local development SMTP configuration")
	}
	auth := identity.New(db, mailer, cfg.SecureCookie)
	mux := http.NewServeMux()
	httpapi.Mount(mux, httpapi.NewPostgresStore(db), logger)
	auth.Mount(mux)
	organizations.New(db, auth, mailer).Mount(mux)
	server := &http.Server{
		Addr:              cfg.Address,
		Handler:           httpapi.Secure(mux, logger),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	result := make(chan error, 1)
	go func() { result <- server.ListenAndServe() }()
	logger.Info("FUTRYVA API starting", "address", cfg.Address)
	select {
	case err := <-result:
		if !errors.Is(err, http.ErrServerClosed) {
			return errors.New("HTTP server failed to start or serve")
		}
		return nil
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdown); err != nil {
			_ = server.Close()
			return errors.New("HTTP graceful shutdown timed out")
		}
		logger.Info("FUTRYVA API stopped gracefully")
		return nil
	}
}
