package organizations

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"futryva/backend/internal/identity"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	errUserHasClub        = errors.New("user already has another active club")
	errClubHasManager     = errors.New("club already has another president")
	errManagerNotVerified = errors.New("manager account is not verified")
)

// SetPlatformOwner is an explicit operator-only bootstrap. Registration never
// grants platform ownership. Repeating the command is harmless and audit-safe.
func SetPlatformOwner(ctx context.Context, db *pgxpool.Pool, email string) error {
	email, valid := normalizeEmail(email)
	if !valid {
		return errors.New("invalid account email")
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		return errors.New("cannot start owner setup")
	}
	defer tx.Rollback(context.Background())
	var id string
	var verified bool
	if err = tx.QueryRow(ctx, `SELECT id::text,email_verified_at IS NOT NULL FROM users WHERE lower(email)=$1 FOR UPDATE`, email).Scan(&id, &verified); errors.Is(err, pgx.ErrNoRows) {
		return errors.New("register the operator account first")
	}
	if err != nil {
		return errors.New("cannot find operator account")
	}
	if !verified {
		return errors.New("verify the operator email before granting ownership")
	}
	tag, err := tx.Exec(ctx, `INSERT INTO platform_owners(user_id) VALUES($1) ON CONFLICT DO NOTHING`, id)
	if err != nil {
		return errors.New("cannot grant platform ownership")
	}
	if tag.RowsAffected() == 1 {
		if _, err = tx.Exec(ctx, `INSERT INTO platform_events(actor_id,action,target_user_id) VALUES($1,'platform_owner_set',$1)`, id); err != nil {
			return errors.New("cannot record owner setup")
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return errors.New("cannot finish owner setup")
	}
	return nil
}

func owner(w http.ResponseWriter, u identity.User) bool {
	if !u.IsPlatformOwner {
		fail(w, 403, "platform_owner_required", "Only the platform owner can provision clubs")
		return false
	}
	return verified(w, u)
}
func assignmentError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errUserHasClub):
		fail(w, 409, "user_has_club", "The account already has another active club")
	case errors.Is(err, errClubHasManager):
		fail(w, 409, "club_has_manager", "This club already has another manager")
	case errors.Is(err, errManagerNotVerified):
		fail(w, 400, "manager_not_verified", "The manager must verify their email first")
	case errors.Is(err, pgx.ErrNoRows):
		fail(w, 404, "manager_not_found", "Register the manager account before assigning a club")
	default:
		dbError(w, err)
	}
}

// assignUser is called only while the target club row is locked. Locking the
// user next also serializes competing assignments involving different clubs.
func assignUser(ctx context.Context, tx pgx.Tx, clubID, userID string, president bool) error {
	var assigned string
	var active, verified bool
	if err := tx.QueryRow(ctx, `SELECT COALESCE(u.assigned_club_id::text,''),
 EXISTS(SELECT 1 FROM clubs c WHERE c.id=u.assigned_club_id AND c.archived_at IS NULL),u.email_verified_at IS NOT NULL
 FROM users u WHERE u.id=$1 FOR UPDATE OF u`, userID).Scan(&assigned, &active, &verified); err != nil {
		return err
	}
	if !verified {
		return errManagerNotVerified
	}
	if active && assigned != clubID {
		return errUserHasClub
	}
	if president {
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM club_memberships m JOIN users incumbent ON incumbent.id=m.user_id WHERE m.club_id=$1 AND m.role='president' AND m.user_id<>$2 AND incumbent.assigned_club_id=m.club_id)`, clubID, userID).Scan(&exists); err != nil {
			return err
		}
		if exists {
			return errClubHasManager
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE users SET assigned_club_id=$2 WHERE id=$1`, userID, clubID); err != nil {
		return err
	}
	if president {
		if _, err := tx.Exec(ctx, `INSERT INTO club_memberships(club_id,user_id,role) VALUES($1,$2,'president') ON CONFLICT DO NOTHING`, clubID, userID); err != nil {
			return err
		}
	}
	return nil
}

type ManagerAccount struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
}
type ManagedClub struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	ShortName string          `json:"shortName"`
	Manager   *ManagerAccount `json:"manager"`
}

const managedClubSelect = `SELECT c.id::text,c.name,COALESCE(c.short_name,''),
 (SELECT jsonb_build_object('id',u.id::text,'email',u.email,'displayName',u.display_name)
 FROM club_memberships m JOIN users u ON u.id=m.user_id
 WHERE m.club_id=c.id AND m.role='president' AND u.assigned_club_id=c.id ORDER BY u.created_at,u.id LIMIT 1)
 FROM clubs c WHERE c.archived_at IS NULL`

func managedClub(ctx context.Context, q queryer, id string) (ManagedClub, error) {
	return scanManagedClub(q.QueryRow(ctx, managedClubSelect+` AND c.id=$1`, id))
}
func scanManagedClub(row pgx.Row) (ManagedClub, error) {
	var c ManagedClub
	var raw []byte
	err := row.Scan(&c.ID, &c.Name, &c.ShortName, &raw)
	if err != nil {
		return c, err
	}
	if len(raw) > 0 {
		c.Manager = &ManagerAccount{}
		err = json.Unmarshal(raw, c.Manager)
	}
	return c, err
}
func (s *Service) adminClubs(w http.ResponseWriter, r *http.Request, u identity.User) {
	if !owner(w, u) {
		return
	}
	rows, err := s.db.Query(r.Context(), managedClubSelect+` ORDER BY c.created_at,c.id LIMIT 100`)
	if err != nil {
		dbError(w, err)
		return
	}
	defer rows.Close()
	out := []ManagedClub{}
	for rows.Next() {
		c, err := scanManagedClub(rows)
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
func (s *Service) adminCreateClub(w http.ResponseWriter, r *http.Request, u identity.User) {
	if !owner(w, u) {
		return
	}
	var in struct {
		Name         string `json:"name"`
		ShortName    string `json:"shortName"`
		ManagerEmail string `json:"managerEmail"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	in.ShortName = strings.TrimSpace(in.ShortName)
	email, valid := normalizeEmail(in.ManagerEmail)
	if !valid || !names(in.Name, in.ShortName) {
		fail(w, 400, "invalid_request", "A valid club name and manager email are required")
		return
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		dbError(w, err)
		return
	}
	defer tx.Rollback(context.Background())
	id := newID()
	if _, err = tx.Exec(r.Context(), `INSERT INTO clubs(id,name,short_name) VALUES($1,$2,$3)`, id, in.Name, in.ShortName); err != nil {
		dbError(w, err)
		return
	}
	var targetID string
	if err = tx.QueryRow(r.Context(), `SELECT id::text FROM users WHERE lower(email)=$1`, email).Scan(&targetID); err != nil {
		assignmentError(w, err)
		return
	}
	if err = assignUser(r.Context(), tx, id, targetID, true); err != nil {
		assignmentError(w, err)
		return
	}
	data, _ := json.Marshal(DefaultTheme())
	if _, err = tx.Exec(r.Context(), `INSERT INTO club_branding_drafts(club_id,theme) VALUES($1,$2)`, id, data); err != nil {
		dbError(w, err)
		return
	}
	if _, err = tx.Exec(r.Context(), `INSERT INTO platform_events(actor_id,action,club_id,target_user_id) VALUES($1,'club_created',$2,$3)`, u.ID, id, targetID); err != nil {
		dbError(w, err)
		return
	}
	c, err := managedClub(r.Context(), tx, id)
	if err != nil {
		dbError(w, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		dbError(w, err)
		return
	}
	jsonResponse(w, 201, map[string]any{"club": c})
}
func (s *Service) adminAssign(w http.ResponseWriter, r *http.Request, u identity.User) {
	if !owner(w, u) {
		return
	}
	id := r.PathValue("clubId")
	if !validID(id) {
		fail(w, 404, "not_found", "Club not found")
		return
	}
	var in struct {
		Email string `json:"email"`
	}
	if !decode(w, r, &in) {
		return
	}
	email, valid := normalizeEmail(in.Email)
	if !valid {
		fail(w, 400, "invalid_request", "Use a valid manager email")
		return
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		dbError(w, err)
		return
	}
	defer tx.Rollback(context.Background())
	var locked string
	if err = tx.QueryRow(r.Context(), `SELECT id::text FROM clubs WHERE id=$1 AND archived_at IS NULL FOR UPDATE`, id).Scan(&locked); err != nil {
		dbError(w, err)
		return
	}
	var targetID string
	if err = tx.QueryRow(r.Context(), `SELECT id::text FROM users WHERE lower(email)=$1`, email).Scan(&targetID); err != nil {
		assignmentError(w, err)
		return
	}
	if err = assignUser(r.Context(), tx, id, targetID, true); err != nil {
		assignmentError(w, err)
		return
	}
	if _, err = tx.Exec(r.Context(), `INSERT INTO platform_events(actor_id,action,club_id,target_user_id) VALUES($1,'club_assigned',$2,$3)`, u.ID, id, targetID); err != nil {
		dbError(w, err)
		return
	}
	c, err := managedClub(r.Context(), tx, id)
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
func (s *Service) adminArchive(w http.ResponseWriter, r *http.Request, u identity.User) {
	if !owner(w, u) {
		return
	}
	id := r.PathValue("clubId")
	if !validID(id) {
		fail(w, 404, "not_found", "Club not found")
		return
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		dbError(w, err)
		return
	}
	defer tx.Rollback(context.Background())
	var locked string
	if err = tx.QueryRow(r.Context(), `SELECT id::text FROM clubs WHERE id=$1 AND archived_at IS NULL FOR UPDATE`, id).Scan(&locked); err != nil {
		dbError(w, err)
		return
	}
	if _, err = tx.Exec(r.Context(), `UPDATE clubs SET archived_at=now() WHERE id=$1`, id); err != nil {
		dbError(w, err)
		return
	}
	if _, err = tx.Exec(r.Context(), `INSERT INTO platform_events(actor_id,action,club_id) VALUES($1,'club_archived',$2)`, u.ID, id); err != nil {
		dbError(w, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		dbError(w, err)
		return
	}
	jsonResponse(w, 204, nil)
}
