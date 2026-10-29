// Package migrate applies additive, versioned SQL without resetting existing data.
package migrate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

var filename = regexp.MustCompile(`^([0-9]{3,})_[a-z0-9_]+\.sql$`)

type migration struct {
	name     string
	version  uint64
	checksum string
	sql      []byte
}

func version(name string) (uint64, error) {
	parts := filename.FindStringSubmatch(name)
	if parts == nil {
		return 0, fmt.Errorf("invalid SQL migration filename %q; use NNN_name.sql", name)
	}
	n, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil || n == 0 {
		return 0, fmt.Errorf("migration %q must have a positive numeric version within uint64 range", name)
	}
	return n, nil
}

// load validates and reads every SQL file before touching the database. Numeric
// ordering remains correct after version 999; duplicate versions are ambiguous.
func load(dir string) ([]migration, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}
	var files []migration
	versions := map[uint64]string{}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.EqualFold(filepath.Ext(name), ".sql") {
			continue
		}
		if entry.IsDir() {
			return nil, fmt.Errorf("SQL migration %q is a directory", name)
		}
		n, err := version(name)
		if err != nil {
			return nil, err
		}
		if previous, ok := versions[n]; ok {
			return nil, fmt.Errorf("duplicate migration version %d: %s and %s", n, previous, name)
		}
		versions[n] = name
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", name, err)
		}
		sum := sha256.Sum256(data)
		files = append(files, migration{name: name, version: n, checksum: hex.EncodeToString(sum[:]), sql: data})
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no SQL migrations in %s", dir)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].version < files[j].version })
	return files, nil
}

// pending validates the complete immutable history before returning any work.
// A newly introduced lower version must never run after newer applied SQL.
func pending(files []migration, applied map[string]string) ([]migration, error) {
	byName := make(map[string]migration, len(files))
	for _, file := range files {
		byName[file.name] = file
	}
	var latest uint64
	for name, checksum := range applied {
		file, ok := byName[name]
		if !ok {
			return nil, fmt.Errorf("applied migration %s missing from directory", name)
		}
		if file.checksum != checksum {
			return nil, fmt.Errorf("migration %s changed after application; add a new migration", name)
		}
		if file.version > latest {
			latest = file.version
		}
	}
	var work []migration
	for _, file := range files {
		if _, ok := applied[file.name]; ok {
			continue
		}
		if file.version < latest {
			return nil, fmt.Errorf("unapplied migration %s precedes applied version %d; add a new higher version", file.name, latest)
		}
		work = append(work, file)
	}
	return work, nil
}

// Apply serializes runners and records immutable checksums in one transaction.
// The original idempotent 001 migration also adopts existing starter databases.
func Apply(ctx context.Context, db *pgxpool.Pool, dir string) error {
	files, err := load(dir)
	if err != nil {
		return err
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin migration: %w", err)
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(785423910)`); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
  name TEXT PRIMARY KEY, checksum TEXT NOT NULL, applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
 )`); err != nil {
		return err
	}
	rows, err := tx.Query(ctx, `SELECT name, checksum FROM schema_migrations`)
	if err != nil {
		return err
	}
	applied := map[string]string{}
	for rows.Next() {
		var name, checksum string
		if err := rows.Scan(&name, &checksum); err != nil {
			rows.Close()
			return err
		}
		applied[name] = checksum
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	work, err := pending(files, applied)
	if err != nil {
		return err
	}
	for _, file := range work {
		if _, err := tx.Exec(ctx, string(file.sql)); err != nil {
			return fmt.Errorf("apply %s: %w", file.name, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations(name,checksum) VALUES($1,$2)`, file.name, file.checksum); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
