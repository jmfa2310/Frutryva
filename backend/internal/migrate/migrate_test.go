package migrate

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const sourceDir = "../../../database"

// Each test owns a separate schema in an explicitly configured test database.
// Schemas are retained: these tests never drop tables or erase existing data.
func testPool(t *testing.T) (context.Context, *pgxpool.Pool) {
	t.Helper()
	url := os.Getenv("DATABASE_TEST_URL")
	if url == "" {
		t.Skip("DATABASE_TEST_URL is unset: PostgreSQL integration test not executed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal("invalid DATABASE_TEST_URL")
	}
	admin, err := pgxpool.NewWithConfig(ctx, cfg.Copy())
	if err != nil {
		t.Fatal("cannot create PostgreSQL test connection")
	}
	defer admin.Close()
	var nonce [12]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatal(err)
	}
	schema := "test_" + hex.EncodeToString(nonce[:])
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); err != nil {
		t.Fatal("cannot create isolated test schema:", err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = pgx.Identifier{schema}.Sanitize()
	db, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal("cannot create isolated PostgreSQL pool")
	}
	t.Cleanup(db.Close)
	t.Log("retained synthetic test schema:", schema)
	return ctx, db
}

func baseline(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(sourceDir, "001_init.sql"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func tempMigrations(t *testing.T, files map[string][]byte) string {
	t.Helper()
	dir := t.TempDir()
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func scalar(t *testing.T, ctx context.Context, db *pgxpool.Pool, sql string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(ctx, sql).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestAdoptsLegacySchemaAndPreservesData(t *testing.T) {
	ctx, db := testPool(t)
	if _, err := db.Exec(ctx, string(baseline(t))); err != nil {
		t.Fatal(err)
	}
	const clubID = "a0000000-0000-4000-8000-000000000001"
	const userID = "b0000000-0000-4000-8000-000000000001"
	if _, err := db.Exec(ctx, `INSERT INTO clubs(id,name) VALUES($1,'Synthetic retained club')`, clubID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO users(id,email,display_name) VALUES($1,'synthetic@example.invalid','Synthetic retained user')`, userID); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := Apply(ctx, db, sourceDir); err != nil {
			t.Fatal(err)
		}
	}
	for query, want := range map[string]int{
		`SELECT count(*) FROM sports`:                                        5,
		`SELECT count(*) FROM sport_variants`:                                7,
		`SELECT count(*) FROM schema_migrations`:                             1,
		`SELECT count(*) FROM clubs WHERE name='Synthetic retained club'`:    1,
		`SELECT count(*) FROM users WHERE email='synthetic@example.invalid'`: 1,
		`SELECT count(*) FROM schema_migrations WHERE name='001_init.sql'`:   1,
		`SELECT count(*) FROM sport_variants v JOIN sports s ON s.id=v.sport_id
		 WHERE (s.code='football' AND v.players_on_field IN (7,9,11))
		 OR (s.code='futsal' AND v.players_on_field=5)
		 OR (s.code='volleyball' AND v.players_on_field=6)
		 OR (s.code='basketball' AND v.players_on_field=5)
		 OR (s.code='handball' AND v.players_on_field=7)`: 7,
	} {
		if got := scalar(t, ctx, db, query); got != want {
			t.Errorf("query %q: got %d, want %d", query, got, want)
		}
	}
}

func TestMigrationRollbackAndImmutableHistory(t *testing.T) {
	ctx, db := testPool(t)
	data := baseline(t)
	if err := Apply(ctx, db, sourceDir); err != nil {
		t.Fatal(err)
	}
	t.Run("failed migration rolls back DDL and ledger", func(t *testing.T) {
		dir := tempMigrations(t, map[string][]byte{
			"001_init.sql":  data,
			"002_probe.sql": []byte(`CREATE TABLE migration_probe(id integer); SELECT 1/0;`),
		})
		if err := Apply(ctx, db, dir); err == nil {
			t.Fatal("expected failing SQL to reject migration")
		}
		var exists bool
		if err := db.QueryRow(ctx, `SELECT to_regclass('migration_probe') IS NOT NULL`).Scan(&exists); err != nil {
			t.Fatal(err)
		}
		if exists {
			t.Error("DDL survived a failed migration")
		}
		if got := scalar(t, ctx, db, `SELECT count(*) FROM schema_migrations`); got != 1 {
			t.Errorf("failed migration changed ledger: %d entries", got)
		}
	})
	t.Run("changed applied file is rejected", func(t *testing.T) {
		dir := tempMigrations(t, map[string][]byte{"001_init.sql": append(append([]byte{}, data...), []byte("\n-- changed\n")...)})
		if err := Apply(ctx, db, dir); err == nil || !strings.Contains(err.Error(), "changed after application") {
			t.Errorf("expected checksum error, got %v", err)
		}
	})
	t.Run("missing applied file is rejected transactionally", func(t *testing.T) {
		dir := tempMigrations(t, map[string][]byte{"002_probe.sql": []byte(`CREATE TABLE migration_probe(id integer);`)})
		if err := Apply(ctx, db, dir); err == nil || !strings.Contains(err.Error(), "missing") {
			t.Errorf("expected missing history error, got %v", err)
		}
		if got := scalar(t, ctx, db, `SELECT count(*) FROM schema_migrations`); got != 1 {
			t.Errorf("rejected history changed ledger: %d entries", got)
		}
	})
}

func TestConcurrentRunnersApplyOnce(t *testing.T) {
	ctx, db := testPool(t)
	var wg sync.WaitGroup
	errs := make(chan error, 3)
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- Apply(ctx, db, sourceDir)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if got := scalar(t, ctx, db, `SELECT count(*) FROM schema_migrations`); got != 1 {
		t.Errorf("concurrent runners recorded %d migrations, want 1", got)
	}
	if got := scalar(t, ctx, db, `SELECT count(*) FROM sport_variants`); got != 7 {
		t.Errorf("concurrent runners seeded %d variants, want 7", got)
	}
}

// This is database referential isolation, not application authorization testing.
// HTTP authorization tests must be added when authenticated routes exist.
func TestCompositeForeignKeysRejectCrossClubRoster(t *testing.T) {
	ctx, db := testPool(t)
	if err := Apply(ctx, db, sourceDir); err != nil {
		t.Fatal(err)
	}
	const clubA = "a0000000-0000-4000-8000-000000000001"
	const clubB = "a0000000-0000-4000-8000-000000000002"
	const teamA = "b0000000-0000-4000-8000-000000000001"
	const playerB = "c0000000-0000-4000-8000-000000000002"
	if _, err := db.Exec(ctx, `INSERT INTO clubs(id,name) VALUES($1,'Synthetic A'),($2,'Synthetic B')`, clubA, clubB); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO teams(id,club_id,sport_variant_id,name)
		VALUES($1,$2,(SELECT min(id) FROM sport_variants),'Synthetic team A')`, teamA, clubA); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO players(id,club_id,display_name) VALUES($1,$2,'Synthetic player B')`, playerB, clubB); err != nil {
		t.Fatal(err)
	}
	for _, club := range []string{clubA, clubB} {
		_, err := db.Exec(ctx, `INSERT INTO team_players(club_id,team_id,player_id) VALUES($1,$2,$3)`, club, teamA, playerB)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23503" {
			t.Errorf("cross-club roster for club %s: expected foreign-key rejection, got %v", club, err)
		}
	}
	if got := scalar(t, ctx, db, `SELECT count(*) FROM team_players`); got != 0 {
		t.Errorf("cross-club writes persisted %d roster entries", got)
	}
	if _, err := db.Exec(ctx, `INSERT INTO players(id,club_id,display_name)
		VALUES('c0000000-0000-4000-8000-000000000001',$1,'Synthetic player A')`, clubA); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO team_players(club_id,team_id,player_id)
		VALUES($1,$2,'c0000000-0000-4000-8000-000000000001')`, clubA, teamA); err != nil {
		t.Fatal("valid same-club roster rejected:", err)
	}
}
