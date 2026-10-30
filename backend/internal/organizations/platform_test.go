package organizations

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"futryva/backend/internal/migrate"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestOnlyExplicitOwnerProvisionsOneClubPerAccount(t *testing.T) {
	f := newFixture(t)
	managerID, manager := f.user(t, "manager@example.invalid", true)
	_, other := f.user(t, "other@example.invalid", true)
	_, unverified := f.user(t, "unverified@example.invalid", false)
	expect(t, f.request("GET", "/v1/admin/clubs", nil, nil), 401)
	expect(t, f.request("GET", "/v1/admin/clubs", manager, nil), 403)
	expect(t, f.request("POST", "/v1/clubs", manager, map[string]any{"name": "Synthetic bypass"}), 403)
	expect(t, f.request("POST", "/v1/clubs", f.owner, map[string]any{"name": "Synthetic old endpoint"}), 403)
	expect(t, f.request("POST", "/v1/admin/clubs", manager, map[string]any{"name": "Synthetic spoof", "managerEmail": "manager@example.invalid", "isPlatformOwner": true}), 403)
	expect(t, f.request("POST", "/v1/admin/clubs", f.owner, map[string]any{"name": "Synthetic unverified", "managerEmail": "unverified@example.invalid"}), 400)
	expect(t, f.request("POST", "/v1/admin/clubs", f.owner, map[string]any{"name": "Synthetic missing", "managerEmail": "missing@example.invalid"}), 404)
	a := f.club(t, manager, "Synthetic managed Alpha")
	b := f.club(t, other, "Synthetic managed Beta")
	expect(t, f.request("GET", "/v1/clubs/"+a.ID, f.owner, nil), 404)
	list := f.request("GET", "/v1/clubs", manager, nil)
	expect(t, list, 200)
	var p struct {
		Clubs []Club `json:"clubs"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &p); err != nil || len(p.Clubs) != 1 || p.Clubs[0].ID != a.ID {
		t.Fatal("manager must see exactly one assigned club", err)
	}
	var before int
	if err := f.db.QueryRow(f.ctx, `SELECT count(*) FROM clubs`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	w := f.request("POST", "/v1/admin/clubs", f.owner, map[string]any{"name": "Synthetic duplicate", "managerEmail": "manager@example.invalid"})
	expect(t, w, 409)
	if !strings.Contains(w.Body.String(), "user_has_club") {
		t.Fatal("wrong assignment conflict", w.Body.String())
	}
	var after int
	if err := f.db.QueryRow(f.ctx, `SELECT count(*) FROM clubs`).Scan(&after); err != nil || after != before {
		t.Fatal("rejected provisioning left a partial club", err)
	}
	expect(t, f.request("POST", "/v1/admin/clubs/"+b.ID+"/assign", f.owner, map[string]any{"email": "manager@example.invalid"}), 409)
	newID, _ := f.user(t, "new-manager@example.invalid", true)
	occupied := f.request("POST", "/v1/admin/clubs/"+a.ID+"/assign", f.owner, map[string]any{"email": "new-manager@example.invalid"})
	expect(t, occupied, 409)
	if !strings.Contains(occupied.Body.String(), "club_has_manager") {
		t.Fatal("existing manager was not protected")
	}
	var assigned *string
	if err := f.db.QueryRow(f.ctx, `SELECT assigned_club_id::text FROM users WHERE id=$1`, newID).Scan(&assigned); err != nil || assigned != nil {
		t.Fatal("failed assignment changed user", err)
	}
	expect(t, f.request("DELETE", "/v1/clubs/"+a.ID, manager, nil), 403)
	expect(t, f.request("DELETE", "/v1/admin/clubs/"+a.ID, f.owner, nil), 204)
	c := f.club(t, manager, "Synthetic new active club")
	if c.ID == a.ID {
		t.Fatal("archive reused historical club")
	}
	var retained int
	if err := f.db.QueryRow(f.ctx, `SELECT count(*) FROM club_memberships WHERE user_id=$1 AND role='president'`, managerID).Scan(&retained); err != nil || retained != 2 {
		t.Fatal("assignment destroyed historical membership", err)
	}
	if err := SetPlatformOwner(f.ctx, f.db, "platform-owner@example.invalid"); err != nil {
		t.Fatal(err)
	}
	var ownerEvents int
	if err := f.db.QueryRow(f.ctx, `SELECT count(*) FROM platform_events WHERE action='platform_owner_set'`).Scan(&ownerEvents); err != nil || ownerEvents != 1 {
		t.Fatal("owner setup must be idempotent", err)
	}
	if err := SetPlatformOwner(f.ctx, f.db, "unverified@example.invalid"); err == nil {
		t.Fatal("unverified owner promotion allowed")
	}
	expect(t, f.request("GET", "/v1/admin/clubs", unverified, nil), 403)
}

func TestInvitationCannotAssignASecondActiveClub(t *testing.T) {
	f := newFixture(t)
	_, managerA := f.user(t, "manager-a@example.invalid", true)
	targetID, managerB := f.user(t, "manager-b@example.invalid", true)
	a := f.club(t, managerA, "Synthetic A")
	b := f.club(t, managerB, "Synthetic B")
	expect(t, f.request("POST", "/v1/clubs/"+a.ID+"/invitations", managerA, map[string]any{"email": "manager-b@example.invalid", "role": "coach"}), 201)
	raw := f.mail.lastToken(t)
	w := f.request("POST", "/v1/invitations/accept", managerB, map[string]any{"token": raw})
	expect(t, w, 409)
	if !strings.Contains(w.Body.String(), "user_has_club") {
		t.Fatal(w.Body.String())
	}
	var current string
	var unused bool
	if err := f.db.QueryRow(f.ctx, `SELECT assigned_club_id::text FROM users WHERE id=$1`, targetID).Scan(&current); err != nil || current != b.ID {
		t.Fatal("cross-club invite changed assignment", err)
	}
	if err := f.db.QueryRow(f.ctx, `SELECT accepted_at IS NULL FROM club_invitations WHERE token_hash=$1`, hashToken(raw)).Scan(&unused); err != nil || !unused {
		t.Fatal("rejected invite was consumed", err)
	}
	expect(t, f.request("GET", "/v1/clubs/"+a.ID, managerB, nil), 404)
}

func TestLegacyMembershipsStayPrivateUntilOwnerChoosesOne(t *testing.T) {
	f := newFixture(t)
	userID, cookie := f.user(t, "legacy-multi@example.invalid", true)
	a, b := newID(), newID()
	for _, id := range []string{a, b} {
		if _, err := f.db.Exec(f.ctx, `INSERT INTO clubs(id,name) VALUES($1,'Synthetic legacy')`, id); err != nil {
			t.Fatal(err)
		}
		if _, err := f.db.Exec(f.ctx, `INSERT INTO club_memberships(club_id,user_id,role) VALUES($1,$2,'president')`, id, userID); err != nil {
			t.Fatal(err)
		}
	}
	w := f.request("GET", "/v1/clubs", cookie, nil)
	expect(t, w, 200)
	var p struct {
		Clubs []Club `json:"clubs"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil || len(p.Clubs) != 0 {
		t.Fatal("ambiguous legacy membership leaked", err)
	}
	expect(t, f.request("GET", "/v1/clubs/"+a, cookie, nil), 404)
	expect(t, f.request("POST", "/v1/admin/clubs/"+a+"/assign", f.owner, map[string]any{"email": "legacy-multi@example.invalid"}), 200)
	expect(t, f.request("GET", "/v1/clubs/"+a, cookie, nil), 200)
	expect(t, f.request("GET", "/v1/clubs/"+b, cookie, nil), 404)
	var count int
	if err := f.db.QueryRow(f.ctx, `SELECT count(*) FROM club_memberships WHERE user_id=$1`, userID).Scan(&count); err != nil || count != 2 {
		t.Fatal("legacy memberships were destroyed", err)
	}
}

func TestManagedClubMigrationPreservesLegacyDataAndAssignsOnlyUnambiguous(t *testing.T) {
	f := newFixture(t)
	cfg := f.db.Config().Copy()
	schema := "legacy_" + strings.ReplaceAll(newID(), "-", "")
	if _, err := f.db.Exec(f.ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = pgx.Identifier{schema}.Sanitize()
	db, err := pgxpool.NewWithConfig(f.ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	first := t.TempDir()
	for _, name := range []string{"001_init.sql", "002_identity_and_clubs.sql"} {
		data, err := os.ReadFile(filepath.Join("../../../database", name))
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(first, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err = migrate.Apply(f.ctx, db, first); err != nil {
		t.Fatal(err)
	}
	single, multi, archived := newID(), newID(), newID()
	a, b, c := newID(), newID(), newID()
	for _, id := range []string{a, b, c} {
		if _, err = db.Exec(f.ctx, `INSERT INTO clubs(id,name) VALUES($1,'Synthetic retained legacy club')`, id); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = db.Exec(f.ctx, `UPDATE clubs SET archived_at=now() WHERE id=$1`, c); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct{ id, email string }{{single, "single@example.invalid"}, {multi, "multi@example.invalid"}, {archived, "archived@example.invalid"}} {
		if _, err = db.Exec(f.ctx, `INSERT INTO users(id,email,display_name) VALUES($1,$2,'Synthetic retained user')`, item.id, item.email); err != nil {
			t.Fatal(err)
		}
	}
	for _, item := range []struct{ club, user, role string }{{a, single, "president"}, {a, single, "coach"}, {a, multi, "coach"}, {b, multi, "coach"}, {c, archived, "president"}} {
		if _, err = db.Exec(f.ctx, `INSERT INTO club_memberships(club_id,user_id,role) VALUES($1,$2,$3)`, item.club, item.user, item.role); err != nil {
			t.Fatal(err)
		}
	}
	if err = migrate.Apply(f.ctx, db, "../../../database"); err != nil {
		t.Fatal(err)
	}
	var assigned string
	if err = db.QueryRow(f.ctx, `SELECT COALESCE(assigned_club_id::text,'') FROM users WHERE id=$1`, single).Scan(&assigned); err != nil || assigned != a {
		t.Fatal("single membership not adopted", err)
	}
	for _, id := range []string{multi, archived} {
		if err = db.QueryRow(f.ctx, `SELECT COALESCE(assigned_club_id::text,'') FROM users WHERE id=$1`, id).Scan(&assigned); err != nil || assigned != "" {
			t.Fatal("ambiguous or archived membership auto-assigned", err)
		}
	}
	var count int
	if err = db.QueryRow(f.ctx, `SELECT count(*) FROM club_memberships`).Scan(&count); err != nil || count != 5 {
		t.Fatal("migration destroyed memberships", err)
	}
	if err = db.QueryRow(f.ctx, `SELECT count(*) FROM platform_owners`).Scan(&count); err != nil || count != 0 {
		t.Fatal("migration granted automatic owner access", err)
	}
}

func TestInactivePresidentsDoNotBlockExplicitAssignmentOrLeakInMembers(t *testing.T) {
	f := newFixture(t)
	legacyA, _ := f.user(t, "legacy-a@example.invalid", true)
	legacyB, _ := f.user(t, "legacy-b@example.invalid", true)
	newManagerID, newManager := f.user(t, "new-manager@example.invalid", true)
	club := newID()
	if _, err := f.db.Exec(f.ctx, `INSERT INTO clubs(id,name) VALUES($1,'Synthetic legacy presidents')`, club); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{legacyA, legacyB} {
		if _, err := f.db.Exec(f.ctx, `INSERT INTO club_memberships(club_id,user_id,role) VALUES($1,$2,'president')`, club, id); err != nil {
			t.Fatal(err)
		}
	}
	expect(t, f.request("POST", "/v1/admin/clubs/"+club+"/assign", f.owner, map[string]any{"email": "new-manager@example.invalid"}), 200)
	w := f.request("GET", "/v1/clubs/"+club+"/members", newManager, nil)
	expect(t, w, 200)
	if strings.Contains(w.Body.String(), "legacy-a@example.invalid") || strings.Contains(w.Body.String(), "legacy-b@example.invalid") {
		t.Fatal("inactive historical memberships exposed personal details")
	}
	if !strings.Contains(w.Body.String(), newManagerID) {
		t.Fatal("active manager missing from members")
	}
	var retained int
	if err := f.db.QueryRow(f.ctx, `SELECT count(*) FROM club_memberships WHERE club_id=$1`, club).Scan(&retained); err != nil || retained != 3 {
		t.Fatal("historical president records were changed", err)
	}
	var assigned int
	if err := f.db.QueryRow(f.ctx, `SELECT count(*) FROM users WHERE id IN($1,$2) AND assigned_club_id IS NOT NULL`, legacyA, legacyB).Scan(&assigned); err != nil || assigned != 0 {
		t.Fatal("legacy users were silently assigned", err)
	}
	// A lower-privilege invitation must not revive a historical president role.
	expect(t, f.request("POST", "/v1/clubs/"+club+"/invitations", newManager, map[string]any{"email": "legacy-a@example.invalid", "role": "coach"}), 409)
}
