package organizations

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"futryva/backend/internal/migrate"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPromotedOwnerCannotManageClubAndHistoricalDataRemain(t *testing.T) {
	f := newFixture(t)
	managerID, manager := f.user(t, "promoted@example.invalid", true)
	c := f.club(t, manager, "Synthetic ACDEA retained club")
	expect(t, f.request("POST", "/v1/clubs/"+c.ID+"/branding/publish", manager, map[string]any{}), 200)
	if err := SetPlatformOwner(f.ctx, f.db, "promoted@example.invalid"); err != nil {
		t.Fatal(err)
	}
	if err := SetPlatformOwner(f.ctx, f.db, "promoted@example.invalid"); err != nil {
		t.Fatal(err)
	}
	var cleared bool
	var events int
	if err := f.db.QueryRow(f.ctx, `SELECT assigned_club_id IS NULL FROM users WHERE id=$1`, managerID).Scan(&cleared); err != nil || !cleared {
		t.Fatal("owner assignment was not removed", err)
	}
	if err := f.db.QueryRow(f.ctx, `SELECT count(*) FROM platform_events WHERE action='owner_club_assignment_removed' AND actor_id=$1 AND club_id=$2`, managerID, c.ID).Scan(&events); err != nil || events != 1 {
		t.Fatal("owner cleanup was not idempotently audited", err)
	}
	for _, suffix := range []string{"", "/branding/draft", "/branding/versions", "/members", "/invitations"} {
		expect(t, f.request("GET", "/v1/clubs/"+c.ID+suffix, manager, nil), 404)
	}
	expect(t, f.request("POST", "/v1/clubs/"+c.ID+"/branding/publish", manager, map[string]any{}), 404)
	empty := f.request("GET", "/v1/clubs", manager, nil)
	expect(t, empty, 200)
	var list struct {
		Clubs []Club `json:"clubs"`
	}
	if err := json.Unmarshal(empty.Body.Bytes(), &list); err != nil || len(list.Clubs) != 0 {
		t.Fatal("owner still sees tenant workspace", err)
	}
	me := f.request("GET", "/v1/me", manager, nil)
	expect(t, me, 200)
	if !strings.Contains(me.Body.String(), `"isPlatformOwner":true`) || strings.Contains(me.Body.String(), `"assignedClubId"`) {
		t.Fatal("owner metadata exposes tenant scope", me.Body.String())
	}
	for _, query := range []string{`SELECT count(*) FROM clubs WHERE id=$1`, `SELECT count(*) FROM club_branding_versions WHERE club_id=$1`, `SELECT count(*) FROM club_branding_drafts WHERE club_id=$1`, `SELECT count(*) FROM club_memberships WHERE club_id=$1 AND role='president'`} {
		var count int
		if err := f.db.QueryRow(f.ctx, query, c.ID).Scan(&count); err != nil || count != 1 {
			t.Fatal("owner separation altered historical tenant data", err)
		}
	}
	var admin struct {
		Clubs []ManagedClub `json:"clubs"`
	}
	w := f.request("GET", "/v1/admin/clubs", manager, nil)
	expect(t, w, 200)
	if err := json.Unmarshal(w.Body.Bytes(), &admin); err != nil || len(admin.Clubs) != 1 || admin.Clubs[0].Manager != nil {
		t.Fatal("former owner-managed club must need a client manager", err)
	}
	// Even an externally stale pointer must never restore a platform-owner role.
	if _, err := f.db.Exec(f.ctx, `UPDATE users SET assigned_club_id=$2 WHERE id=$1`, managerID, c.ID); err != nil {
		t.Fatal(err)
	}
	expect(t, f.request("GET", "/v1/clubs/"+c.ID, manager, nil), 404)
	empty = f.request("GET", "/v1/clubs", manager, nil)
	expect(t, empty, 200)
	if strings.Contains(empty.Body.String(), c.ID) {
		t.Fatal("stale owner pointer leaked tenant")
	}
	_, client := f.user(t, "replacement@example.invalid", true)
	expect(t, f.request("POST", "/v1/admin/clubs/"+c.ID+"/assign", manager, map[string]any{"email": "replacement@example.invalid"}), 200)
	expect(t, f.request("GET", "/v1/clubs/"+c.ID, client, nil), 200)
	members := f.request("GET", "/v1/clubs/"+c.ID+"/members", client, nil)
	expect(t, members, 200)
	if strings.Contains(members.Body.String(), "promoted@example.invalid") {
		t.Fatal("historical owner listed as customer member")
	}
}

func TestOwnerAssignmentAndInvitationAreRejectedWithoutPartialChanges(t *testing.T) {
	f := newFixture(t)
	_, manager := f.user(t, "manager@example.invalid", true)
	c := f.club(t, manager, "Synthetic independent client club")
	var before int
	if err := f.db.QueryRow(f.ctx, `SELECT count(*) FROM clubs`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	w := f.request("POST", "/v1/admin/clubs", f.owner, map[string]any{"name": "Synthetic forbidden owner club", "managerEmail": "platform-owner@example.invalid"})
	expect(t, w, 409)
	if !strings.Contains(w.Body.String(), "platform_owner_cannot_manage_club") {
		t.Fatal(w.Body.String())
	}
	var after int
	if err := f.db.QueryRow(f.ctx, `SELECT count(*) FROM clubs`).Scan(&after); err != nil || before != after {
		t.Fatal("owner assignment left orphan club", err)
	}
	w = f.request("POST", "/v1/admin/clubs/"+c.ID+"/assign", f.owner, map[string]any{"email": "platform-owner@example.invalid"})
	expect(t, w, 409)
	if !strings.Contains(w.Body.String(), "platform_owner_cannot_manage_club") {
		t.Fatal(w.Body.String())
	}
	expect(t, f.request("POST", "/v1/clubs/"+c.ID+"/invitations", manager, map[string]any{"email": "platform-owner@example.invalid", "role": "coach"}), 201)
	raw := f.mail.lastToken(t)
	w = f.request("POST", "/v1/invitations/accept", f.owner, map[string]any{"token": raw})
	expect(t, w, 409)
	if !strings.Contains(w.Body.String(), "platform_owner_cannot_manage_club") {
		t.Fatal(w.Body.String())
	}
	var unused bool
	if err := f.db.QueryRow(f.ctx, `SELECT accepted_at IS NULL FROM club_invitations WHERE token_hash=$1`, hashToken(raw)).Scan(&unused); err != nil || !unused {
		t.Fatal("owner invitation consumed on rejection", err)
	}
}

func TestOwnerSeparationMigrationPreservesClubBrandingAndMembership(t *testing.T) {
	f := newFixture(t)
	cfg := f.db.Config().Copy()
	schema := "owner_legacy_" + strings.ReplaceAll(newID(), "-", "")
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
	for _, name := range []string{"001_init.sql", "002_identity_and_clubs.sql", "003_managed_club_assignment.sql"} {
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
	user, club := newID(), newID()
	if _, err = db.Exec(f.ctx, `INSERT INTO users(id,email,display_name,email_verified_at) VALUES($1,'owner-legacy@example.invalid','Synthetic owner',now())`, user); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(f.ctx, `INSERT INTO clubs(id,name) VALUES($1,'Synthetic ACDEA history')`, club); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(f.ctx, `INSERT INTO club_memberships(club_id,user_id,role) VALUES($1,$2,'president')`, club, user); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(f.ctx, `UPDATE users SET assigned_club_id=$2 WHERE id=$1`, user, club); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(f.ctx, `INSERT INTO platform_owners(user_id) VALUES($1)`, user); err != nil {
		t.Fatal(err)
	}
	theme, _ := json.Marshal(DefaultTheme())
	if _, err = db.Exec(f.ctx, `INSERT INTO club_branding_drafts(club_id,theme) VALUES($1,$2)`, club, theme); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(f.ctx, `INSERT INTO club_branding_versions(club_id,version,theme,published_by) VALUES($1,1,$2,$3)`, club, theme, user); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(f.ctx, `UPDATE clubs SET published_branding_version=1 WHERE id=$1`, club); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err = migrate.Apply(f.ctx, db, "../../../database"); err != nil {
			t.Fatal(err)
		}
	}
	var cleared bool
	if err = db.QueryRow(f.ctx, `SELECT assigned_club_id IS NULL FROM users WHERE id=$1`, user).Scan(&cleared); err != nil || !cleared {
		t.Fatal("migration failed to separate owner", err)
	}
	var count int
	if err = db.QueryRow(f.ctx, `SELECT count(*) FROM platform_events WHERE action='owner_club_assignment_removed' AND actor_id=$1 AND club_id=$2 AND target_user_id=$1`, user, club).Scan(&count); err != nil || count != 1 {
		t.Fatal("migration owner separation audit wrong", err)
	}
	for _, query := range []string{`SELECT count(*) FROM clubs WHERE id=$1 AND name='Synthetic ACDEA history'`, `SELECT count(*) FROM club_memberships WHERE club_id=$1`, `SELECT count(*) FROM club_branding_versions WHERE club_id=$1`, `SELECT count(*) FROM club_branding_drafts WHERE club_id=$1`} {
		if err = db.QueryRow(f.ctx, query, club).Scan(&count); err != nil || count != 1 {
			t.Fatal("migration damaged preserved tenant history", err)
		}
	}
}

// This exercises READ COMMITTED snapshots: the assigner starts its locking
// SELECT before promotion commits, waits, and must check ownership afterwards.
func TestAssignmentRechecksOwnerAfterWaitingForUserLock(t *testing.T) {
	f := newFixture(t)
	target, _ := f.user(t, "racing-owner@example.invalid", true)
	club := newID()
	if _, err := f.db.Exec(f.ctx, `INSERT INTO clubs(id,name) VALUES($1,'Synthetic ownership race')`, club); err != nil {
		t.Fatal(err)
	}
	promotion, err := f.db.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer promotion.Rollback(f.ctx)
	var locked string
	if err = promotion.QueryRow(f.ctx, `SELECT id::text FROM users WHERE id=$1 FOR UPDATE`, target).Scan(&locked); err != nil {
		t.Fatal(err)
	}
	if _, err = promotion.Exec(f.ctx, `INSERT INTO platform_owners(user_id) VALUES($1)`, target); err != nil {
		t.Fatal(err)
	}
	pid := make(chan int, 1)
	done := make(chan error, 1)
	go func() {
		tx, err := f.db.Begin(f.ctx)
		if err != nil {
			done <- err
			return
		}
		defer tx.Rollback(f.ctx)
		var backend int
		if err = tx.QueryRow(f.ctx, `SELECT pg_backend_pid()`).Scan(&backend); err != nil {
			done <- err
			return
		}
		pid <- backend
		var id string
		if err = tx.QueryRow(f.ctx, `SELECT id::text FROM clubs WHERE id=$1 FOR NO KEY UPDATE`, club).Scan(&id); err != nil {
			done <- err
			return
		}
		done <- assignUser(f.ctx, tx, club, target, true)
	}()
	var backend int
	select {
	case backend = <-pid:
	case <-f.ctx.Done():
		t.Fatal("assignment did not start")
	}
	waiting := false
	for attempt := 0; attempt < 100; attempt++ {
		if err = f.db.QueryRow(f.ctx, `SELECT COALESCE(wait_event_type='Lock',false) FROM pg_stat_activity WHERE pid=$1`, backend).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !waiting {
		t.Fatal("assignment did not wait for the promoted user lock")
	}
	if err = promotion.Commit(f.ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err = <-done:
		if !errors.Is(err, errOwnerCannotManage) {
			t.Fatalf("assignment failed to refresh platform-owner status: %v", err)
		}
	case <-f.ctx.Done():
		t.Fatal("assignment remained blocked")
	}
	var unassigned bool
	if err = f.db.QueryRow(f.ctx, `SELECT assigned_club_id IS NULL FROM users WHERE id=$1`, target).Scan(&unassigned); err != nil || !unassigned {
		t.Fatal("promoted owner gained tenant scope", err)
	}
}
