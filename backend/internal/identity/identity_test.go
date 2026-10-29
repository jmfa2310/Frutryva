package identity

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
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

	"futryva/backend/internal/migrate"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type sentMessage struct{ to, subject, body string }
type capturedMail struct {
	mu       sync.Mutex
	messages []sentMessage
	fail     bool
}

func (m *capturedMail) Send(_ context.Context, to, subject, body string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fail {
		return errors.New("synthetic delivery failure")
	}
	m.messages = append(m.messages, sentMessage{to, subject, body})
	return nil
}
func (m *capturedMail) latestToken(t *testing.T) string {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.messages) == 0 {
		t.Fatal("no account email delivered")
	}
	body := m.messages[len(m.messages)-1].body
	match := regexp.MustCompile(`\b[A-Za-z0-9_-]{43}\b`).FindString(body)
	if !validToken(match) {
		t.Fatal("email did not contain a usable one-time token")
	}
	return match
}

func apiRequest(t *testing.T, handler http.Handler, method, path string, body any, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(method, path, bytes.NewReader(data))
	r.Header.Set("Content-Type", "application/json")
	for _, cookie := range cookies {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w
}
func status(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Fatalf("status=%d want=%d response=%s", w.Code, want, w.Body.String())
	}
}
func responseUser(t *testing.T, w *httptest.ResponseRecorder) User {
	t.Helper()
	var body struct {
		User User `json:"user"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body.User
}
func sessionCookie(t *testing.T, w *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == CookieName {
			return cookie
		}
	}
	t.Fatal("missing session cookie")
	return nil
}

func integrationService(t *testing.T) (context.Context, *Service, *capturedMail, http.Handler) {
	t.Helper()
	url := os.Getenv("DATABASE_TEST_URL")
	if url == "" {
		t.Skip("DATABASE_TEST_URL unset: identity PostgreSQL integration not executed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal("invalid DATABASE_TEST_URL")
	}
	admin, err := pgxpool.NewWithConfig(ctx, cfg.Copy())
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	var random [12]byte
	if _, err := rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	schema := "identity_test_" + hex.EncodeToString(random[:])
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = pgx.Identifier{schema}.Sanitize()
	db, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	if err := migrate.Apply(ctx, db, "../../../database"); err != nil {
		t.Fatal(err)
	}
	mail := &capturedMail{}
	service := New(db, mail, false)
	mux := http.NewServeMux()
	service.Mount(mux)
	t.Log("retained synthetic identity schema:", schema)
	return ctx, service, mail, mux
}

func TestAccountInputAndRandomSecrets(t *testing.T) {
	for input, want := range map[string]string{"PERSON@Example.Invalid": "person@example.invalid", " person@example.invalid ": "person@example.invalid"} {
		got, err := normalizedEmail(input)
		if err != nil || got != want {
			t.Fatalf("email normalization failed for %q", input)
		}
	}
	for _, email := range []string{"not-an-email", "A Person <person@example.invalid>", "person@example.invalid\nBcc: person2@example.invalid", strings.Repeat("a", 250) + "@example.invalid"} {
		if _, err := normalizedEmail(email); err == nil {
			t.Errorf("invalid email accepted: %q", email)
		}
	}
	for _, tc := range []struct {
		password string
		valid    bool
	}{{strings.Repeat("a", 11), false}, {strings.Repeat("a", 12), true}, {strings.Repeat("a", 72), true}, {strings.Repeat("a", 73), false}, {strings.Repeat("á", 37), false}} {
		if validPassword(tc.password) != tc.valid {
			t.Error("password byte-length policy mismatch")
		}
	}
	first, err := randomToken()
	if err != nil {
		t.Fatal(err)
	}
	second, err := randomToken()
	if err != nil {
		t.Fatal(err)
	}
	if !validToken(first) || first == second || hashToken(first) == first || len(hashToken(first)) != 64 {
		t.Fatal("invalid token generation/hash")
	}
	for _, raw := range []string{"", strings.Repeat("a", 42), "invalid+token", strings.Repeat("a", 44)} {
		if validToken(raw) {
			t.Fatal("malformed token accepted")
		}
	}
	id, err := randomID()
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`).MatchString(id) {
		t.Fatal("invalid random UUID")
	}
}

func TestIdentityCookiesAndInputErrors(t *testing.T) {
	service := New(nil, nil, true)
	w := httptest.NewRecorder()
	service.setCookie(w, "synthetic-session")
	cookie := sessionCookie(t, w)
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode || cookie.Path != "/" || cookie.MaxAge != 86400 {
		t.Fatal("unsafe session cookie options")
	}
	w = httptest.NewRecorder()
	service.clearCookie(w)
	if sessionCookie(t, w).MaxAge != -1 {
		t.Fatal("logout did not expire cookie")
	}
	mux := http.NewServeMux()
	service.Mount(mux)
	status(t, apiRequest(t, mux, "GET", "/v1/auth/login", nil), 405)
	status(t, apiRequest(t, mux, "GET", "/v1/me", nil), 401)
	status(t, apiRequest(t, mux, "POST", "/v1/auth/verify-email", map[string]string{"token": "bad"}), 400)
	status(t, apiRequest(t, mux, "POST", "/v1/auth/reset-password", map[string]string{"token": "bad", "password": "too-short"}), 400)
	status(t, apiRequest(t, mux, "POST", "/v1/auth/register", map[string]string{"email": "invalid", "displayName": "A", "password": "short"}), 400)
	for _, body := range []string{`{"email":"a@example.invalid","password":"synthetic-password","unknown":true}`, `{"email":1}`, `{} {}`, `{"email":"` + strings.Repeat("a", 17<<10) + `"}`} {
		r := httptest.NewRequest("POST", "/v1/auth/login", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		status(t, w, 400)
	}
	r := httptest.NewRequest("POST", "/v1/auth/login", strings.NewReader("{}"))
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	status(t, w, 415)
	status(t, apiRequest(t, mux, "POST", "/v1/auth/logout", nil), 204)
}

func TestIdentitySessionLifecycleWithPostgres(t *testing.T) {
	ctx, service, mail, handler := integrationService(t)
	const password = "Synthetic-initial-password-42"
	const replacement = "Synthetic-replaced-password-43"
	registration := apiRequest(t, handler, "POST", "/v1/auth/register", map[string]string{"email": " PERSON@Example.Invalid ", "displayName": "Synthetic Person", "password": password})
	status(t, registration, 201)
	user := responseUser(t, registration)
	if user.Email != "person@example.invalid" || user.EmailVerified || user.ID == "" {
		t.Fatal("invalid registered account response")
	}
	cookie := sessionCookie(t, registration)
	if !cookie.HttpOnly || cookie.Secure || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatal("incorrect development cookie")
	}
	var storedPassword, storedToken string
	if err := service.db.QueryRow(ctx, `SELECT password_hash FROM users WHERE id=$1`, user.ID).Scan(&storedPassword); err != nil {
		t.Fatal(err)
	}
	if err := service.db.QueryRow(ctx, `SELECT token_hash FROM auth_sessions WHERE user_id=$1`, user.ID).Scan(&storedToken); err != nil {
		t.Fatal(err)
	}
	if storedPassword == password || bcrypt.CompareHashAndPassword([]byte(storedPassword), []byte(password)) != nil || storedToken != hashToken(cookie.Value) {
		t.Fatal("password or session was not safely hashed")
	}
	cost, err := bcrypt.Cost([]byte(storedPassword))
	if err != nil || cost != passwordCost {
		t.Fatal("incorrect password hashing cost")
	}
	status(t, apiRequest(t, handler, "GET", "/v1/me", nil, cookie), 200)
	verification := mail.latestToken(t)
	status(t, apiRequest(t, handler, "POST", "/v1/auth/verify-email", map[string]string{"token": verification}), 204)
	status(t, apiRequest(t, handler, "POST", "/v1/auth/verify-email", map[string]string{"token": verification}), 400)
	me := apiRequest(t, handler, "GET", "/v1/me", nil, cookie)
	status(t, me, 200)
	if !responseUser(t, me).EmailVerified {
		t.Fatal("email verification not persisted")
	}
	duplicate := apiRequest(t, handler, "POST", "/v1/auth/register", map[string]string{"email": "PERSON@example.invalid", "displayName": "Another Synthetic Person", "password": password})
	status(t, duplicate, 409)
	wrong := apiRequest(t, handler, "POST", "/v1/auth/login", map[string]string{"email": user.Email, "password": "Synthetic-wrong-password"})
	unknown := apiRequest(t, handler, "POST", "/v1/auth/login", map[string]string{"email": "absent@example.invalid", "password": "Synthetic-wrong-password"})
	status(t, wrong, 401)
	status(t, unknown, 401)
	if wrong.Body.String() != unknown.Body.String() {
		t.Fatal("login exposed whether account exists")
	}
	refresh := apiRequest(t, handler, "POST", "/v1/auth/refresh", nil, cookie)
	status(t, refresh, 200)
	rotated := sessionCookie(t, refresh)
	if rotated.Value == cookie.Value {
		t.Fatal("refresh did not rotate token")
	}
	status(t, apiRequest(t, handler, "GET", "/v1/me", nil, cookie), 401)
	status(t, apiRequest(t, handler, "POST", "/v1/auth/refresh", nil, cookie), 401)
	status(t, apiRequest(t, handler, "GET", "/v1/me", nil, rotated), 200)
	status(t, apiRequest(t, handler, "POST", "/v1/auth/logout", nil, rotated), 204)
	status(t, apiRequest(t, handler, "GET", "/v1/me", nil, rotated), 401)
	login := apiRequest(t, handler, "POST", "/v1/auth/login", map[string]string{"email": user.Email, "password": password})
	status(t, login, 200)
	active := sessionCookie(t, login)
	forgotKnown := apiRequest(t, handler, "POST", "/v1/auth/forgot-password", map[string]string{"email": user.Email})
	resetToken := mail.latestToken(t)
	forgotUnknown := apiRequest(t, handler, "POST", "/v1/auth/forgot-password", map[string]string{"email": "absent@example.invalid"})
	status(t, forgotKnown, 202)
	status(t, forgotUnknown, 202)
	if forgotKnown.Body.String() != forgotUnknown.Body.String() {
		t.Fatal("password recovery exposed whether account exists")
	}
	status(t, apiRequest(t, handler, "POST", "/v1/auth/verify-email", map[string]string{"token": resetToken}), 400)
	status(t, apiRequest(t, handler, "POST", "/v1/auth/reset-password", map[string]string{"token": resetToken, "password": replacement}), 204)
	status(t, apiRequest(t, handler, "GET", "/v1/me", nil, active), 401)
	status(t, apiRequest(t, handler, "POST", "/v1/auth/reset-password", map[string]string{"token": resetToken, "password": replacement}), 400)
	status(t, apiRequest(t, handler, "POST", "/v1/auth/login", map[string]string{"email": user.Email, "password": password}), 401)
	status(t, apiRequest(t, handler, "POST", "/v1/auth/login", map[string]string{"email": user.Email, "password": replacement}), 200)
}

func TestIdentityExpiryAndDeliveryRecoveryWithPostgres(t *testing.T) {
	ctx, service, mail, handler := integrationService(t)
	mail.fail = true
	registration := apiRequest(t, handler, "POST", "/v1/auth/register", map[string]string{"email": "retry@example.invalid", "displayName": "Synthetic Retry", "password": "Synthetic-password-retry"})
	status(t, registration, 201)
	if !strings.Contains(registration.Body.String(), `"emailDeliveryPending":true`) {
		t.Fatal("mail failure did not expose recoverable delivery state")
	}
	cookie := sessionCookie(t, registration)
	mail.fail = false
	status(t, apiRequest(t, handler, "POST", "/v1/auth/resend-verification", map[string]string{}, cookie), 202)
	token := mail.latestToken(t)
	if _, err := service.db.Exec(ctx, `UPDATE auth_tokens SET expires_at=now()-interval '1 second' WHERE token_hash=$1`, hashToken(token)); err != nil {
		t.Fatal(err)
	}
	status(t, apiRequest(t, handler, "POST", "/v1/auth/verify-email", map[string]string{"token": token}), 400)
	if _, err := service.db.Exec(ctx, `UPDATE auth_sessions SET expires_at=now()-interval '1 second' WHERE token_hash=$1`, hashToken(cookie.Value)); err != nil {
		t.Fatal(err)
	}
	status(t, apiRequest(t, handler, "GET", "/v1/me", nil, cookie), 401)
	status(t, apiRequest(t, handler, "POST", "/v1/auth/refresh", nil, cookie), 401)
}
