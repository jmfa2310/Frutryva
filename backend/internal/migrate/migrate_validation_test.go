package migrate

import (
	"strings"
	"testing"
)

func TestLoadSortsVersionsNumerically(t *testing.T) {
	dir := tempMigrations(t, map[string][]byte{
		"1000_next.sql":  []byte("SELECT 1;"),
		"999_before.sql": []byte("SELECT 1;"),
		"001_init.sql":   []byte("SELECT 1;"),
		"README.md":      []byte("Migration documentation"),
	})
	files, err := load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 3 {
		t.Fatalf("got %d SQL files", len(files))
	}
	for i, want := range []uint64{1, 999, 1000} {
		if files[i].version != want {
			t.Fatalf("position %d version=%d want=%d", i, files[i].version, want)
		}
		if len(files[i].checksum) != 64 {
			t.Fatal("missing SHA256 checksum")
		}
	}
}

func TestLoadRejectsAmbiguousOrMalformedSQL(t *testing.T) {
	for _, tc := range []struct {
		name    string
		files   map[string][]byte
		message string
	}{
		{"no SQL", map[string][]byte{"README.md": nil}, "no SQL migrations"},
		{"unversioned SQL", map[string][]byte{"schema.sql": nil}, "invalid SQL migration filename"},
		{"short version", map[string][]byte{"01_init.sql": nil}, "invalid SQL migration filename"},
		{"uppercase extension", map[string][]byte{"001_init.SQL": nil}, "invalid SQL migration filename"},
		{"uppercase name", map[string][]byte{"001_INIT.sql": nil}, "invalid SQL migration filename"},
		{"zero version", map[string][]byte{"000_init.sql": nil}, "positive numeric version"},
		{"version overflow", map[string][]byte{"18446744073709551616_init.sql": nil}, "positive numeric version"},
		{"same version different names", map[string][]byte{"002_a.sql": nil, "002_b.sql": nil}, "duplicate migration version"},
		{"same version different padding", map[string][]byte{"001_init.sql": nil, "0001_again.sql": nil}, "duplicate migration version"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := load(tempMigrations(t, tc.files))
			if err == nil || !strings.Contains(err.Error(), tc.message) {
				t.Fatalf("expected %q, got %v", tc.message, err)
			}
		})
	}
}

func TestPendingValidatesAllHistoryBeforeSQLExecution(t *testing.T) {
	files := []migration{{name: "001_init.sql", version: 1, checksum: "one"}, {name: "003_later.sql", version: 3, checksum: "three"}}
	for _, tc := range []struct {
		name    string
		applied map[string]string
		want    string
	}{
		{"missing history", map[string]string{"002_missing.sql": "two"}, "missing"},
		{"changed checksum", map[string]string{"001_init.sql": "changed"}, "changed after application"},
		{"retroactive version", map[string]string{"003_later.sql": "three"}, "precedes applied version"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			work, err := pending(files, tc.applied)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q, got %v", tc.want, err)
			}
			if work != nil {
				t.Fatal("invalid history produced executable migrations")
			}
		})
	}
	work, err := pending(files, map[string]string{"001_init.sql": "one"})
	if err != nil || len(work) != 1 || work[0].version != 3 {
		t.Fatalf("valid next migration rejected: %v", err)
	}
}

func TestNumericOrderingAndRetroactiveVersionsWithPostgres(t *testing.T) {
	ctx, db := testPool(t)
	files := map[string][]byte{
		"001_init.sql":   baseline(t),
		"999_before.sql": []byte("CREATE TABLE numeric_order_probe(value integer NOT NULL);"),
		"1000_after.sql": []byte("INSERT INTO numeric_order_probe(value) VALUES(1000);"),
	}
	dir := tempMigrations(t, files)
	if err := Apply(ctx, db, dir); err != nil {
		t.Fatal("numeric migration ordering failed:", err)
	}
	if got := scalar(t, ctx, db, "SELECT value FROM numeric_order_probe"); got != 1000 {
		t.Fatal("later migration did not execute")
	}
	files["500_retroactive.sql"] = []byte("INSERT INTO numeric_order_probe(value) VALUES(500);")
	if err := Apply(ctx, db, tempMigrations(t, files)); err == nil || !strings.Contains(err.Error(), "precedes applied version") {
		t.Fatalf("retroactive migration was not rejected: %v", err)
	}
	if got := scalar(t, ctx, db, "SELECT count(*) FROM numeric_order_probe"); got != 1 {
		t.Fatal("rejected migration changed data")
	}
	if got := scalar(t, ctx, db, "SELECT count(*) FROM schema_migrations"); got != 3 {
		t.Fatalf("rejected migration changed history: %d records", got)
	}
}
