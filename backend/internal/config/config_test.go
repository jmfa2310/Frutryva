package config

import (
	"strings"
	"testing"
)

func clearConfig(t *testing.T) {
	t.Helper()
	for _, name := range []string{"DATABASE_URL", "PGHOST", "PORT", "HOST", "MIGRATIONS_DIR"} {
		t.Setenv(name, "")
	}
}

func TestLoadRequiresExplicitDatabaseConfiguration(t *testing.T) {
	clearConfig(t)
	if _, err := Load(); err == nil {
		t.Fatal("expected configuration error")
	}
}

func TestLoadPostgresEnvironmentAndDefaults(t *testing.T) {
	clearConfig(t)
	t.Setenv("PGHOST", "db")
	t.Setenv("PGPASSWORD", "test@password:with/special#characters")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DatabaseURL != "" {
		t.Fatal("PG* configuration should be consumed directly by pgx")
	}
	if cfg.Address != ":8080" || cfg.MigrationsDir != "../database" {
		t.Fatalf("unexpected defaults: address=%q migrations=%q", cfg.Address, cfg.MigrationsDir)
	}
	if cfg.StartupTimeout <= 0 || cfg.ShutdownTimeout <= 0 {
		t.Fatal("missing timeouts")
	}
}

func TestLoadOverrides(t *testing.T) {
	clearConfig(t)
	t.Setenv("DATABASE_URL", "postgres://database.invalid/test")
	t.Setenv("HOST", "127.0.0.1")
	t.Setenv("PORT", "9090")
	t.Setenv("MIGRATIONS_DIR", "/app/database")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Address != "127.0.0.1:9090" || cfg.MigrationsDir != "/app/database" {
		t.Fatal("explicit settings were not used")
	}
}

func TestLoadRejectsInvalidPortsWithoutLeakingValues(t *testing.T) {
	for _, port := range []string{"0", "65536", "-1", "bad-secret-value", "1:8080"} {
		t.Run(port, func(t *testing.T) {
			clearConfig(t)
			t.Setenv("PGHOST", "db")
			t.Setenv("PORT", port)
			_, err := Load()
			if err == nil {
				t.Fatal("expected validation error")
			}
			if strings.Contains(err.Error(), port) {
				t.Fatal("error included raw invalid setting")
			}
		})
	}
}
