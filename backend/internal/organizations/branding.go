package organizations

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"futryva/backend/internal/identity"
	"github.com/jackc/pgx/v5"
)

func draftFor(ctx context.Context, q queryer, userID, clubID string) (Theme, error) {
	var raw []byte
	err := q.QueryRow(ctx, `SELECT COALESCE(d.theme,jsonb_build_object(
 'primaryColor',trim(c.primary_color),'secondaryColor',trim(c.secondary_color),'accentColor',trim(c.accent_color),
 'surfaceColor','#07172B','textColor','#FFFFFF','logoUrl',COALESCE(c.logo_url,''),'coverUrl',COALESCE(c.cover_url,''),
 'sponsors','[]'::jsonb,'kitColors','[]'::jsonb))
 FROM clubs c LEFT JOIN club_branding_drafts d ON d.club_id=c.id WHERE c.id=$1 AND c.archived_at IS NULL AND c.id=(SELECT assigned_club_id FROM users WHERE id=$2)
 AND EXISTS(SELECT 1 FROM club_memberships m WHERE m.club_id=c.id AND m.user_id=$2 AND m.role IN('president','director'))`, clubID, userID).Scan(&raw)
	if err != nil {
		return Theme{}, err
	}
	var theme Theme
	err = json.Unmarshal(raw, &theme)
	return theme, err
}
func (s *Service) getDraft(w http.ResponseWriter, r *http.Request, u identity.User) {
	c, ok := s.authorized(w, r, u, true)
	if !ok {
		return
	}
	theme, err := draftFor(r.Context(), s.db, u.ID, c.ID)
	if err != nil {
		dbError(w, err)
		return
	}
	jsonResponse(w, 200, map[string]any{"theme": theme})
}
func (s *Service) putDraft(w http.ResponseWriter, r *http.Request, u identity.User) {
	var in struct {
		Theme Theme `json:"theme"`
	}
	if !decode(w, r, &in) {
		return
	}
	if err := validateTheme(&in.Theme); err != nil {
		fail(w, 400, "invalid_theme", err.Error())
		return
	}
	tx, c, ok := s.transaction(w, r, u)
	if !ok {
		return
	}
	defer tx.Rollback(context.Background())
	data, _ := json.Marshal(in.Theme)
	if _, err := tx.Exec(r.Context(), `INSERT INTO club_branding_drafts(club_id,theme) VALUES($1,$2) ON CONFLICT(club_id) DO UPDATE SET theme=EXCLUDED.theme,updated_at=now()`, c.ID, data); err != nil {
		dbError(w, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		dbError(w, err)
		return
	}
	jsonResponse(w, 200, map[string]any{"theme": in.Theme})
}

// publishTheme always appends a new immutable version while the club row is locked.
func publishTheme(ctx context.Context, tx pgx.Tx, c Club, u identity.User, theme Theme) (Club, int, error) {
	if err := validateTheme(&theme); err != nil {
		return c, 0, err
	}
	data, _ := json.Marshal(theme)
	var version int
	if err := tx.QueryRow(ctx, `SELECT COALESCE(max(version),0)+1 FROM club_branding_versions WHERE club_id=$1`, c.ID).Scan(&version); err != nil {
		return c, 0, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO club_branding_versions(club_id,version,theme,published_by) VALUES($1,$2,$3,$4)`, c.ID, version, data, u.ID); err != nil {
		return c, 0, err
	}
	if _, err := tx.Exec(ctx, `UPDATE clubs SET published_branding_version=$2 WHERE id=$1`, c.ID, version); err != nil {
		return c, 0, err
	}
	c.Branding = &theme
	c.BrandingVersion = version
	return c, version, nil
}
func (s *Service) publish(w http.ResponseWriter, r *http.Request, u identity.User) {
	tx, c, ok := s.transaction(w, r, u)
	if !ok {
		return
	}
	defer tx.Rollback(context.Background())
	theme, err := draftFor(r.Context(), tx, u.ID, c.ID)
	if err != nil {
		dbError(w, err)
		return
	}
	if err = validateTheme(&theme); err != nil {
		fail(w, 400, "invalid_theme", err.Error())
		return
	}
	c, version, err := publishTheme(r.Context(), tx, c, u, theme)
	if err != nil {
		dbError(w, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		dbError(w, err)
		return
	}
	jsonResponse(w, 200, map[string]any{"club": c, "version": version})
}
func (s *Service) versions(w http.ResponseWriter, r *http.Request, u identity.User) {
	c, ok := s.authorized(w, r, u, true)
	if !ok {
		return
	}
	rows, err := s.db.Query(r.Context(), `SELECT v.version,v.theme,v.created_at FROM club_branding_versions v
 WHERE v.club_id=$1 AND v.club_id=(SELECT assigned_club_id FROM users WHERE id=$2) AND EXISTS(SELECT 1 FROM club_memberships m WHERE m.club_id=v.club_id AND m.user_id=$2 AND m.role IN('president','director')) ORDER BY v.version DESC LIMIT 100`, c.ID, u.ID)
	if err != nil {
		dbError(w, err)
		return
	}
	defer rows.Close()
	type version struct {
		Version   int       `json:"version"`
		Theme     Theme     `json:"theme"`
		CreatedAt time.Time `json:"createdAt"`
	}
	out := []version{}
	for rows.Next() {
		var v version
		var raw []byte
		if err := rows.Scan(&v.Version, &raw, &v.CreatedAt); err != nil {
			dbError(w, err)
			return
		}
		if err := json.Unmarshal(raw, &v.Theme); err != nil {
			dbError(w, err)
			return
		}
		out = append(out, v)
	}
	if err = rows.Err(); err != nil {
		dbError(w, err)
		return
	}
	jsonResponse(w, 200, map[string]any{"versions": out})
}
func (s *Service) rollback(w http.ResponseWriter, r *http.Request, u identity.User) {
	var in struct {
		Version int `json:"version"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Version < 1 {
		fail(w, 400, "invalid_request", "Select a published version")
		return
	}
	tx, c, ok := s.transaction(w, r, u)
	if !ok {
		return
	}
	defer tx.Rollback(context.Background())
	var raw []byte
	if err := tx.QueryRow(r.Context(), `SELECT theme FROM club_branding_versions WHERE club_id=$1 AND version=$2`, c.ID, in.Version).Scan(&raw); err != nil {
		dbError(w, err)
		return
	}
	var theme Theme
	if err := json.Unmarshal(raw, &theme); err != nil {
		dbError(w, err)
		return
	}
	c, version, err := publishTheme(r.Context(), tx, c, u, theme)
	if err != nil {
		dbError(w, err)
		return
	}
	if _, err = tx.Exec(r.Context(), `INSERT INTO club_branding_drafts(club_id,theme) VALUES($1,$2) ON CONFLICT(club_id) DO UPDATE SET theme=EXCLUDED.theme,updated_at=now()`, c.ID, raw); err != nil {
		dbError(w, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		dbError(w, err)
		return
	}
	jsonResponse(w, 200, map[string]any{"club": c, "version": version})
}
func (s *Service) publicClub(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("clubId")
	if !validID(id) {
		fail(w, 404, "not_found", "Club not found")
		return
	}
	var c Club
	var raw []byte
	err := s.db.QueryRow(r.Context(), `SELECT c.id::text,c.name,COALESCE(c.short_name,''),v.theme,v.version FROM clubs c
 JOIN club_branding_versions v ON v.club_id=c.id AND v.version=c.published_branding_version WHERE c.id=$1 AND c.archived_at IS NULL`, id).Scan(&c.ID, &c.Name, &c.ShortName, &raw, &c.BrandingVersion)
	if err != nil {
		dbError(w, err)
		return
	}
	c.Branding = &Theme{}
	if err = json.Unmarshal(raw, c.Branding); err != nil {
		dbError(w, err)
		return
	}
	jsonResponse(w, 200, map[string]any{"club": c})
}
func (s *Service) members(w http.ResponseWriter, r *http.Request, u identity.User) {
	c, ok := s.authorized(w, r, u, true)
	if !ok {
		return
	}
	rows, err := s.db.Query(r.Context(), `SELECT m.user_id::text,u.display_name,u.email,m.role FROM club_memberships m JOIN users u ON u.id=m.user_id
 WHERE m.club_id=$1 AND u.assigned_club_id=m.club_id AND m.club_id=(SELECT assigned_club_id FROM users WHERE id=$2) AND EXISTS(SELECT 1 FROM club_memberships mine WHERE mine.club_id=m.club_id AND mine.user_id=$2 AND mine.role IN('president','director')) ORDER BY u.display_name,m.role LIMIT 100`, c.ID, u.ID)
	if err != nil {
		dbError(w, err)
		return
	}
	defer rows.Close()
	type member struct {
		UserID      string `json:"userId"`
		DisplayName string `json:"displayName"`
		Email       string `json:"email"`
		Role        string `json:"role"`
	}
	out := []member{}
	for rows.Next() {
		var m member
		if err := rows.Scan(&m.UserID, &m.DisplayName, &m.Email, &m.Role); err != nil {
			dbError(w, err)
			return
		}
		out = append(out, m)
	}
	if err = rows.Err(); err != nil {
		dbError(w, err)
		return
	}
	jsonResponse(w, 200, map[string]any{"members": out})
}
