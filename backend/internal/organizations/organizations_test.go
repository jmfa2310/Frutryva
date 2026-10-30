package organizations

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"futryva/backend/internal/identity"
	"futryva/backend/internal/migrate"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type sentMail struct{ to, body string }
type captureMailer struct {
	mu       sync.Mutex
	messages []sentMail
	fail     bool
}

func (m *captureMailer) Send(_ context.Context, to, subject, body string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fail {
		return errors.New("synthetic email failure")
	}
	m.messages = append(m.messages, sentMail{to, body})
	return nil
}
func (m *captureMailer) lastToken(t *testing.T) string {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.messages) == 0 {
		t.Fatal("no invitation email")
	}
	code := regexp.MustCompile(`[A-Za-z0-9_-]{43}`).FindString(m.messages[len(m.messages)-1].body)
	if len(code) != 43 {
		t.Fatal("no one-use token in email")
	}
	return code
}

type orgFixture struct {
	db    *pgxpool.Pool
	ctx   context.Context
	h     http.Handler
	mail  *captureMailer
	s     *Service
	owner *http.Cookie
}

func newFixture(t *testing.T) *orgFixture {
	t.Helper()
	dsn := os.Getenv("DATABASE_TEST_URL")
	if dsn == "" {
		t.Skip("DATABASE_TEST_URL unset: organization PostgreSQL tests not executed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	t.Cleanup(cancel)
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal("invalid test database configuration")
	}
	admin, err := pgxpool.NewWithConfig(ctx, cfg.Copy())
	if err != nil {
		t.Fatal(err)
	}
	schema := "org_" + strings.ReplaceAll(newID(), "-", "")
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	admin.Close()
	cfg.ConnConfig.RuntimeParams["search_path"] = pgx.Identifier{schema}.Sanitize()
	db, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	if err = migrate.Apply(ctx, db, "../../../database"); err != nil {
		t.Fatal(err)
	}
	mailer := &captureMailer{}
	auth := identity.New(db, mailer, false)
	s := New(db, auth, mailer)
	mux := http.NewServeMux()
	s.Mount(mux)
	auth.Mount(mux)
	f := &orgFixture{db: db, ctx: ctx, h: mux, mail: mailer, s: s}
	_, f.owner = f.user(t, "platform-owner@example.invalid", true)
	if err := SetPlatformOwner(ctx, db, "platform-owner@example.invalid"); err != nil {
		t.Fatal(err)
	}
	return f
}
func (f *orgFixture) user(t *testing.T, email string, verified bool) (string, *http.Cookie) {
	t.Helper()
	id := newID()
	var verifiedAt *time.Time
	if verified {
		now := time.Now()
		verifiedAt = &now
	}
	if _, err := f.db.Exec(f.ctx, `INSERT INTO users(id,email,display_name,email_verified_at) VALUES($1,$2,'Synthetic test user',$3)`, id, email, verifiedAt); err != nil {
		t.Fatal(err)
	}
	raw, hash, err := token()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.db.Exec(f.ctx, `INSERT INTO auth_sessions(token_hash,user_id,expires_at) VALUES($1,$2,now()+interval '1 day')`, hash, id); err != nil {
		t.Fatal(err)
	}
	return id, &http.Cookie{Name: identity.CookieName, Value: raw}
}
func (f *orgFixture) request(method, path string, cookie *http.Cookie, body any) *httptest.ResponseRecorder {
	var b []byte
	if body != nil {
		b, _ = json.Marshal(body)
	}
	r := httptest.NewRequest(method, path, bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	f.h.ServeHTTP(w, r)
	return w
}
func expect(t *testing.T, w *httptest.ResponseRecorder, status int) {
	t.Helper()
	if w.Code != status {
		t.Fatalf("status got%d want%d body%s", w.Code, status, w.Body.String())
	}
}
func (f *orgFixture) club(t *testing.T, cookie *http.Cookie, name string) Club {
	t.Helper()
	var email string
	if err := f.db.QueryRow(f.ctx, `SELECT u.email FROM users u JOIN auth_sessions s ON s.user_id=u.id WHERE s.token_hash=$1`, hashToken(cookie.Value)).Scan(&email); err != nil {
		t.Fatal(err)
	}
	w := f.request("POST", "/v1/admin/clubs", f.owner, map[string]any{"name": name, "shortName": "SYN", "managerEmail": email})
	expect(t, w, 201)
	var p struct {
		Club ManagedClub `json:"club"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	private := f.request("GET", "/v1/clubs/"+p.Club.ID, cookie, nil)
	expect(t, private, 200)
	return parseClub(t, private)
}
func parseClub(t *testing.T, w *httptest.ResponseRecorder) Club {
	t.Helper()
	var p struct {
		Club Club `json:"club"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	return p.Club
}

func TestClubsIsolateMembershipAndPublishedBranding(t *testing.T) {
	f := newFixture(t)
	_, president := f.user(t, "president@example.invalid", true)
	_, outsider := f.user(t, "outsider@example.invalid", true)
	coachID, coach := f.user(t, "coach@example.invalid", true)
	a := f.club(t, president, "Synthetic Alpha")
	_, presidentB := f.user(t, "president-b@example.invalid", true)
	b := f.club(t, presidentB, "Synthetic Beta")
	f.club(t, outsider, "Synthetic outsider")
	expect(t, f.request("GET", "/v1/clubs/"+a.ID, outsider, nil), 404)
	expect(t, f.request("GET", "/v1/clubs/"+a.ID+"/branding/draft", outsider, nil), 404)
	expect(t, f.request("GET", "/v1/clubs/not-a-uuid", president, nil), 404)
	expect(t, f.request("GET", "/v1/clubs", nil, nil), 401)
	list := f.request("GET", "/v1/clubs", president, nil)
	expect(t, list, 200)
	if strings.Contains(list.Body.String(), "Synthetic outsider") {
		t.Fatal("cross-club list leaked")
	}
	expect(t, f.request("GET", "/v1/public/clubs/"+a.ID, nil, nil), 404)
	for _, c := range []Club{a, b} {
		theme := DefaultTheme()
		managerCookie := president
		if c.ID == b.ID {
			managerCookie = presidentB
			theme.PrimaryColor = "#ff6600"
		}
		expect(t, f.request("PUT", "/v1/clubs/"+c.ID+"/branding/draft", managerCookie, map[string]any{"theme": theme}), 200)
		expect(t, f.request("POST", "/v1/clubs/"+c.ID+"/branding/publish", managerCookie, nil), 200)
	}
	publicA := parseClub(t, f.request("GET", "/v1/public/clubs/"+a.ID, nil, nil))
	publicB := parseClub(t, f.request("GET", "/v1/public/clubs/"+b.ID, nil, nil))
	if publicA.Branding.PrimaryColor == publicB.Branding.PrimaryColor || publicA.Role != "" {
		t.Fatal("public branding wrong or private role exposed")
	}
	next := DefaultTheme()
	next.PrimaryColor = "#aa3377"
	expect(t, f.request("PUT", "/v1/clubs/"+a.ID+"/branding/draft", president, map[string]any{"theme": next}), 200)
	before := parseClub(t, f.request("GET", "/v1/public/clubs/"+a.ID, nil, nil))
	if before.Branding.PrimaryColor == next.PrimaryColor {
		t.Fatal("draft leaked publicly")
	}
	expect(t, f.request("POST", "/v1/clubs/"+a.ID+"/branding/publish", president, nil), 200)
	rolled := f.request("POST", "/v1/clubs/"+a.ID+"/branding/rollback", president, map[string]any{"version": 1})
	expect(t, rolled, 200)
	after := parseClub(t, rolled)
	if after.BrandingVersion != 3 || after.Branding.PrimaryColor != publicA.Branding.PrimaryColor {
		t.Fatal("rollback failed to append immutable version")
	}
	if _, err := f.db.Exec(f.ctx, `INSERT INTO club_memberships(club_id,user_id,role) VALUES($1,$2,'coach')`, a.ID, coachID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(f.ctx, `UPDATE users SET assigned_club_id=$2 WHERE id=$1`, coachID, a.ID); err != nil {
		t.Fatal(err)
	}
	expect(t, f.request("GET", "/v1/clubs/"+a.ID, coach, nil), 200)
	expect(t, f.request("PUT", "/v1/clubs/"+a.ID+"/branding/draft", coach, map[string]any{"theme": next}), 403)
	expect(t, f.request("GET", "/v1/clubs/"+a.ID+"/members", coach, nil), 403)
	expect(t, f.request("GET", "/v1/clubs/"+a.ID+"/invitations", coach, nil), 403)
	expect(t, f.request("PATCH", "/v1/clubs/"+a.ID, president, map[string]any{"name": "Synthetic renamed", "shortName": "REN"}), 200)
	expect(t, f.request("DELETE", "/v1/clubs/"+a.ID, president, nil), 403)
	expect(t, f.request("DELETE", "/v1/admin/clubs/"+a.ID, f.owner, nil), 204)
	expect(t, f.request("GET", "/v1/public/clubs/"+a.ID, nil, nil), 404)
	expect(t, f.request("GET", "/v1/clubs/"+a.ID, president, nil), 404)
	var retained bool
	if err := f.db.QueryRow(f.ctx, `SELECT archived_at IS NOT NULL FROM clubs WHERE id=$1`, a.ID).Scan(&retained); err != nil || !retained {
		t.Fatal("archive failed to retain club row", err)
	}
}

func TestInvitationExpiryEmailOwnershipReplayAndRoles(t *testing.T) {
	f := newFixture(t)
	_, president := f.user(t, "president@example.invalid", true)
	targetID, target := f.user(t, "target@example.invalid", true)
	_, other := f.user(t, "other@example.invalid", true)
	directorID, director := f.user(t, "director@example.invalid", true)
	c := f.club(t, president, "Synthetic invite club")
	path := "/v1/clubs/" + c.ID + "/invitations"
	body := map[string]any{"email": "target@example.invalid", "role": "coach"}
	w := f.request("POST", path, president, body)
	expect(t, w, 201)
	raw := f.mail.lastToken(t)
	if strings.Contains(w.Body.String(), raw) {
		t.Fatal("raw token leaked in HTTP response")
	}
	expect(t, f.request("POST", path, president, body), 409)
	expect(t, f.request("POST", "/v1/invitations/accept", other, map[string]any{"token": raw}), 400)
	var stored string
	if err := f.db.QueryRow(f.ctx, `SELECT token_hash FROM club_invitations WHERE club_id=$1`, c.ID).Scan(&stored); err != nil || stored != hashToken(raw) {
		t.Fatal("invitation token not hashed", err)
	}
	if _, err := f.db.Exec(f.ctx, `UPDATE club_invitations SET expires_at=now()-interval '1 second' WHERE token_hash=$1`, stored); err != nil {
		t.Fatal(err)
	}
	expect(t, f.request("POST", "/v1/invitations/accept", target, map[string]any{"token": raw}), 400)
	expect(t, f.request("POST", path, president, body), 201)
	raw = f.mail.lastToken(t)
	accepted := f.request("POST", "/v1/invitations/accept", target, map[string]any{"token": raw})
	expect(t, accepted, 200)
	if parseClub(t, accepted).Role != "coach" {
		t.Fatal("invitation role not applied")
	}
	expect(t, f.request("POST", "/v1/invitations/accept", target, map[string]any{"token": raw}), 400)
	var memberships int
	if err := f.db.QueryRow(f.ctx, `SELECT count(*) FROM club_memberships WHERE club_id=$1 AND user_id=$2`, c.ID, targetID).Scan(&memberships); err != nil || memberships != 1 {
		t.Fatal("replay duplicated membership", err)
	}
	if _, err := f.db.Exec(f.ctx, `INSERT INTO club_memberships(club_id,user_id,role) VALUES($1,$2,'director')`, c.ID, directorID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(f.ctx, `UPDATE users SET assigned_club_id=$2 WHERE id=$1`, directorID, c.ID); err != nil {
		t.Fatal(err)
	}
	expect(t, f.request("POST", path, director, map[string]any{"email": "new-director@example.invalid", "role": "director"}), 403)
	expect(t, f.request("DELETE", "/v1/clubs/"+c.ID, director, nil), 403)
	revoked := f.request("POST", path, president, map[string]any{"email": "other@example.invalid", "role": "assistant"})
	expect(t, revoked, 201)
	raw = f.mail.lastToken(t)
	var p struct {
		Invitation Invitation `json:"invitation"`
	}
	if err := json.Unmarshal(revoked.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	expect(t, f.request("DELETE", path+"/"+p.Invitation.ID, president, nil), 204)
	expect(t, f.request("POST", "/v1/invitations/accept", other, map[string]any{"token": raw}), 400)
}

func TestConcurrentInvitationAcceptanceRunsOnce(t *testing.T) {
	f := newFixture(t)
	_, president := f.user(t, "president@example.invalid", true)
	_, target := f.user(t, "target@example.invalid", true)
	c := f.club(t, president, "Synthetic concurrency")
	expect(t, f.request("POST", "/v1/clubs/"+c.ID+"/invitations", president, map[string]any{"email": "target@example.invalid", "role": "coach"}), 201)
	raw := f.mail.lastToken(t)
	results := make(chan int, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- f.request("POST", "/v1/invitations/accept", target, map[string]any{"token": raw}).Code
		}()
	}
	wg.Wait()
	close(results)
	counts := map[int]int{}
	for status := range results {
		counts[status]++
	}
	if counts[200] != 1 || counts[400] != 1 {
		t.Fatal("concurrent token replay not rejected", counts)
	}
}

func TestUnverifiedWritesInvalidThemeAndEmailFailure(t *testing.T) {
	f := newFixture(t)
	_, unverified := f.user(t, "unverified@example.invalid", false)
	expect(t, f.request("POST", "/v1/clubs", unverified, map[string]any{"name": "Synthetic club"}), 403)
	_, president := f.user(t, "president@example.invalid", true)
	c := f.club(t, president, "Synthetic invalid input")
	theme := DefaultTheme()
	theme.LogoURL = "javascript:alert(1)"
	expect(t, f.request("PUT", "/v1/clubs/"+c.ID+"/branding/draft", president, map[string]any{"theme": theme}), 400)
	expect(t, f.request("POST", "/v1/admin/clubs", f.owner, map[string]any{"name": "Synthetic", "unknown": true}), 400)
	f.mail.fail = true
	path := "/v1/clubs/" + c.ID + "/invitations"
	body := map[string]any{"email": "retry@example.invalid", "role": "coach"}
	expect(t, f.request("POST", path, president, body), 503)
	var pending int
	if err := f.db.QueryRow(f.ctx, `SELECT count(*) FROM club_invitations WHERE revoked_at IS NULL`).Scan(&pending); err != nil || pending != 0 {
		t.Fatal("failed email left active undeliverable invitation", err)
	}
	f.mail.fail = false
	expect(t, f.request("POST", path, president, body), 201)
}
