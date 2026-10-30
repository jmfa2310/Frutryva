// Package organizations owns club membership, invitations and published branding.
package organizations

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/mail"
	"strings"
	"unicode/utf8"

	"futryva/backend/internal/identity"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	db     *pgxpool.Pool
	auth   *identity.Service
	mailer identity.Mailer
}

func New(db *pgxpool.Pool, auth *identity.Service, mailer identity.Mailer) *Service {
	return &Service{db: db, auth: auth, mailer: mailer}
}

type Club struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	ShortName       string `json:"shortName"`
	Role            string `json:"role,omitempty"`
	Branding        *Theme `json:"branding"`
	BrandingVersion int    `json:"brandingVersion"`
	Archived        bool   `json:"archived"`
}

type queryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

const clubSelect = `SELECT c.id::text,c.name,COALESCE(c.short_name,''),
 (SELECT m.role FROM club_memberships m WHERE m.club_id=c.id AND m.user_id=$1
 ORDER BY CASE m.role WHEN 'president' THEN 0 WHEN 'director' THEN 1 WHEN 'coach' THEN 2 ELSE 3 END LIMIT 1),
 v.theme,COALESCE(c.published_branding_version,0)
 FROM clubs c LEFT JOIN club_branding_versions v ON v.club_id=c.id AND v.version=c.published_branding_version
 WHERE c.archived_at IS NULL AND NOT EXISTS(SELECT 1 FROM platform_owners p WHERE p.user_id=$1) AND c.id=(SELECT assigned_club_id FROM users WHERE id=$1)
 AND EXISTS(SELECT 1 FROM club_memberships m WHERE m.club_id=c.id AND m.user_id=$1)`

func scanClub(row pgx.Row) (Club, error) {
	var c Club
	var raw []byte
	err := row.Scan(&c.ID, &c.Name, &c.ShortName, &c.Role, &raw, &c.BrandingVersion)
	if err != nil {
		return c, err
	}
	if len(raw) > 0 {
		c.Branding = &Theme{}
		if err = json.Unmarshal(raw, c.Branding); err != nil {
			return c, err
		}
	}
	return c, nil
}
func clubFor(ctx context.Context, q queryer, userID, clubID string, lock bool) (Club, error) {
	sql := clubSelect + ` AND c.id=$2`
	if lock {
		sql += ` FOR NO KEY UPDATE OF c`
	}
	return scanClub(q.QueryRow(ctx, sql, userID, clubID))
}
func manager(role string) bool { return role == "president" || role == "director" }
func (s *Service) Mount(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/admin/clubs", s.auth.Require(s.adminClubs))
	mux.HandleFunc("POST /v1/admin/clubs", s.auth.Require(s.adminCreateClub))
	mux.HandleFunc("POST /v1/admin/clubs/{clubId}/assign", s.auth.Require(s.adminAssign))
	mux.HandleFunc("DELETE /v1/admin/clubs/{clubId}", s.auth.Require(s.adminArchive))
	mux.HandleFunc("GET /v1/clubs", s.auth.Require(s.listClubs))
	mux.HandleFunc("POST /v1/clubs", s.auth.Require(s.createClub))
	mux.HandleFunc("GET /v1/clubs/{clubId}", s.auth.Require(s.getClub))
	mux.HandleFunc("PATCH /v1/clubs/{clubId}", s.auth.Require(s.updateClub))
	mux.HandleFunc("DELETE /v1/clubs/{clubId}", s.auth.Require(s.archiveClub))
	mux.HandleFunc("GET /v1/clubs/{clubId}/branding/draft", s.auth.Require(s.getDraft))
	mux.HandleFunc("PUT /v1/clubs/{clubId}/branding/draft", s.auth.Require(s.putDraft))
	mux.HandleFunc("POST /v1/clubs/{clubId}/branding/publish", s.auth.Require(s.publish))
	mux.HandleFunc("GET /v1/clubs/{clubId}/branding/versions", s.auth.Require(s.versions))
	mux.HandleFunc("POST /v1/clubs/{clubId}/branding/rollback", s.auth.Require(s.rollback))
	mux.HandleFunc("GET /v1/clubs/{clubId}/members", s.auth.Require(s.members))
	mux.HandleFunc("GET /v1/clubs/{clubId}/invitations", s.auth.Require(s.listInvitations))
	mux.HandleFunc("POST /v1/clubs/{clubId}/invitations", s.auth.Require(s.invite))
	mux.HandleFunc("DELETE /v1/clubs/{clubId}/invitations/{invitationId}", s.auth.Require(s.revokeInvitation))
	mux.HandleFunc("POST /v1/invitations/accept", s.auth.Require(s.acceptInvitation))
	mux.HandleFunc("GET /v1/public/clubs/{clubId}", s.publicClub)
}

func jsonResponse(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}
func fail(w http.ResponseWriter, status int, code, message string) {
	jsonResponse(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if r.Header.Get("Content-Type") != "" && !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		fail(w, 415, "unsupported_media_type", "Use application/json")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		fail(w, 400, "invalid_request", "Invalid JSON body")
		return false
	}
	if err := d.Decode(new(any)); err != io.EOF {
		fail(w, 400, "invalid_request", "Only one JSON value is allowed")
		return false
	}
	return true
}
func verified(w http.ResponseWriter, u identity.User) bool {
	if !u.EmailVerified {
		fail(w, 403, "email_not_verified", "Verify your email before making changes")
		return false
	}
	return true
}
func validID(id string) bool {
	if len(id) != 36 {
		return false
	}
	for i, c := range id {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
			continue
		}
		if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
			return false
		}
	}
	return true
}
func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("crypto random unavailable")
	}
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	x := hex.EncodeToString(b[:])
	return x[:8] + "-" + x[8:12] + "-" + x[12:16] + "-" + x[16:20] + "-" + x[20:]
}
func token() (string, string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", "", err
	}
	raw := base64.RawURLEncoding.EncodeToString(b[:])
	return raw, hashToken(raw), nil
}
func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
func normalizeEmail(s string) (string, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	a, e := mail.ParseAddress(s)
	return s, e == nil && a.Address == s && len(s) <= 254 && !strings.ContainsAny(s, "\r\n")
}
func names(name, short string) bool {
	return utf8.RuneCountInString(name) >= 2 && utf8.RuneCountInString(name) <= 120 && utf8.RuneCountInString(short) <= 30 && !strings.ContainsAny(name+short, "\x00\r\n")
}
func dbError(w http.ResponseWriter, err error) {
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 404, "not_found", "Club or resource not found")
		return
	}
	var p *pgconn.PgError
	if errors.As(err, &p) && p.Code == "23505" {
		fail(w, 409, "conflict", "Resource already exists")
		return
	}
	fail(w, 500, "database_error", "Unable to complete this operation")
}
func (s *Service) authorized(w http.ResponseWriter, r *http.Request, u identity.User, manage bool) (Club, bool) {
	id := r.PathValue("clubId")
	if !validID(id) {
		fail(w, 404, "not_found", "Club not found")
		return Club{}, false
	}
	c, err := clubFor(r.Context(), s.db, u.ID, id, false)
	if err != nil {
		dbError(w, err)
		return Club{}, false
	}
	if manage && !manager(c.Role) {
		fail(w, 403, "forbidden", "Club management permission required")
		return Club{}, false
	}
	return c, true
}
func (s *Service) transaction(w http.ResponseWriter, r *http.Request, u identity.User) (pgx.Tx, Club, bool) {
	if !verified(w, u) {
		return nil, Club{}, false
	}
	id := r.PathValue("clubId")
	if !validID(id) {
		fail(w, 404, "not_found", "Club not found")
		return nil, Club{}, false
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		dbError(w, err)
		return nil, Club{}, false
	}
	c, err := clubFor(r.Context(), tx, u.ID, id, true)
	if err != nil {
		_ = tx.Rollback(r.Context())
		dbError(w, err)
		return nil, Club{}, false
	}
	// Serialize tenant writes with promotion and assignment. Re-read scope
	// with a fresh statement after acquiring the user lock.
	var lockedUser string
	if err = tx.QueryRow(r.Context(), `SELECT id::text FROM users WHERE id=$1 FOR UPDATE`, u.ID).Scan(&lockedUser); err != nil {
		_ = tx.Rollback(r.Context())
		dbError(w, err)
		return nil, Club{}, false
	}
	c, err = clubFor(r.Context(), tx, u.ID, id, false)
	if err != nil {
		_ = tx.Rollback(r.Context())
		dbError(w, err)
		return nil, Club{}, false
	}
	if !manager(c.Role) {
		_ = tx.Rollback(r.Context())
		fail(w, 403, "forbidden", "Club management permission required")
		return nil, Club{}, false
	}
	return tx, c, true
}
func (s *Service) listClubs(w http.ResponseWriter, r *http.Request, u identity.User) {
	rows, err := s.db.Query(r.Context(), clubSelect+` ORDER BY c.created_at,c.id LIMIT 100`, u.ID)
	if err != nil {
		dbError(w, err)
		return
	}
	defer rows.Close()
	out := []Club{}
	for rows.Next() {
		c, err := scanClub(rows)
		if err != nil {
			dbError(w, err)
			return
		}
		out = append(out, c)
	}
	if err = rows.Err(); err != nil {
		dbError(w, err)
		return
	}
	jsonResponse(w, 200, map[string]any{"clubs": out})
}
func (s *Service) getClub(w http.ResponseWriter, r *http.Request, u identity.User) {
	c, ok := s.authorized(w, r, u, false)
	if ok {
		jsonResponse(w, 200, map[string]any{"club": c})
	}
}

// Club provisioning belongs exclusively to the explicit platform-owner API.
func (s *Service) createClub(w http.ResponseWriter, r *http.Request, u identity.User) {
	fail(w, 403, "platform_owner_required", "Club accounts are provisioned by the platform owner")
}
func (s *Service) updateClub(w http.ResponseWriter, r *http.Request, u identity.User) {
	var in struct {
		Name      string `json:"name"`
		ShortName string `json:"shortName"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	in.ShortName = strings.TrimSpace(in.ShortName)
	if !names(in.Name, in.ShortName) {
		fail(w, 400, "invalid_request", "Invalid club name")
		return
	}
	tx, c, ok := s.transaction(w, r, u)
	if !ok {
		return
	}
	defer tx.Rollback(context.Background())
	if _, err := tx.Exec(r.Context(), `UPDATE clubs SET name=$2,short_name=$3 WHERE id=$1`, c.ID, in.Name, in.ShortName); err != nil {
		dbError(w, err)
		return
	}
	c, err := clubFor(r.Context(), tx, u.ID, c.ID, false)
	if err != nil {
		dbError(w, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		dbError(w, err)
		return
	}
	jsonResponse(w, 200, map[string]any{"club": c})
}
func (s *Service) archiveClub(w http.ResponseWriter, r *http.Request, u identity.User) {
	fail(w, 403, "platform_owner_required", "Only the platform owner can archive a club")
}
