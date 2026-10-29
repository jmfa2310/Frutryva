// Package config loads process configuration without exposing credentials.
package config

import (
	"errors"
	"net"
	"os"
	"strconv"
	"time"
)

type Config struct {
	DatabaseURL     string
	Address         string
	MigrationsDir   string
	StartupTimeout  time.Duration
	ShutdownTimeout time.Duration
}

// Load accepts either DATABASE_URL or standard PostgreSQL PG* variables.
// pgx reads PGHOST, PGPORT, PGDATABASE, PGUSER, PGPASSWORD and PGSSLMODE itself;
// passwords are never interpolated into a URI.
func Load() (Config, error) {
	c := Config{DatabaseURL: os.Getenv("DATABASE_URL"), MigrationsDir: os.Getenv("MIGRATIONS_DIR"), StartupTimeout: 30 * time.Second, ShutdownTimeout: 10 * time.Second}
	if c.DatabaseURL == "" && os.Getenv("PGHOST") == "" {
		return Config{}, errors.New("configure DATABASE_URL or PGHOST and the PostgreSQL PG* variables")
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return Config{}, errors.New("PORT must be an integer between 1 and 65535")
	}
	c.Address = net.JoinHostPort(os.Getenv("HOST"), port)
	if c.MigrationsDir == "" {
		c.MigrationsDir = "../database"
	}
	return c, nil
}
