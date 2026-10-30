package organizations

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"futryva/backend/internal/identity"
	"github.com/jackc/pgx/v5"
)

type Invitation struct {
	ID         string     `json:"id"`
	Email      string     `json:"email"`
	Role       string     `json:"role"`
	ExpiresAt  time.Time  `json:"expiresAt"`
	AcceptedAt *time.Time `json:"acceptedAt"`
	RevokedAt  *time.Time `json:"revokedAt"`
}

func scanInvitation(row pgx.Row) (Invitation, error) {
	var i Invitation
	err := row.Scan(&i.ID, &i.Email, &i.Role, &i.ExpiresAt, &i.AcceptedAt, &i.RevokedAt)
	return i, err
}

const invitationFields = `id::text,email,role,expires_at,accepted_at,revoked_at`

func (s *Service) listInvitations(w http.ResponseWriter, r *http.Request, u identity.User) {
	c, ok := s.authorized(w, r, u, true)
	if !ok {
		return
	}
	rows, err := s.db.Query(r.Context(), `SELECT `+invitationFields+` FROM club_invitations i WHERE club_id=$1 AND club_id=(SELECT assigned_club_id FROM users WHERE id=$2)
 AND EXISTS(SELECT 1 FROM club_memberships m WHERE m.club_id=i.club_id AND m.user_id=$2 AND m.role IN('president','director')) ORDER BY created_at DESC LIMIT 100`, c.ID, u.ID)
	if err != nil {
		dbError(w, err)
		return
	}
	defer rows.Close()
	out := []Invitation{}
	for rows.Next() {
		i, err := scanInvitation(rows)
		if err != nil {
			dbError(w, err)
			return
		}
		out = append(out, i)
	}
	if err = rows.Err(); err != nil {
		dbError(w, err)
		return
	}
	jsonResponse(w, 200, map[string]any{"invitations": out})
}
func (s *Service) invite(w http.ResponseWriter, r *http.Request, u identity.User) {
	var in struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if !decode(w, r, &in) {
		return
	}
	email, valid := normalizeEmail(in.Email)
	if !valid || (in.Role != "coach" && in.Role != "assistant" && in.Role != "director") {
		fail(w, 400, "invalid_request", "Use a valid email and coach, assistant or director role")
		return
	}
	tx, c, ok := s.transaction(w, r, u)
	if !ok {
		return
	}
	defer tx.Rollback(context.Background())
	if in.Role == "director" && c.Role != "president" {
		fail(w, 403, "forbidden", "Only the president may invite a director")
		return
	}
	if s.mailer == nil {
		fail(w, 503, "email_unavailable", "Email delivery is temporarily unavailable")
		return
	}
	var member bool
	if err := tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM club_memberships m JOIN users u ON u.id=m.user_id WHERE m.club_id=$1 AND lower(u.email)=$2)`, c.ID, email).Scan(&member); err != nil {
		dbError(w, err)
		return
	}
	if member {
		fail(w, 409, "already_member", "This user is already a club member")
		return
	}
	if _, err := tx.Exec(r.Context(), `UPDATE club_invitations SET revoked_at=now() WHERE club_id=$1 AND lower(email)=$2 AND accepted_at IS NULL AND revoked_at IS NULL AND expires_at<=now()`, c.ID, email); err != nil {
		dbError(w, err)
		return
	}
	var pending bool
	if err := tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM club_invitations WHERE club_id=$1 AND lower(email)=$2 AND accepted_at IS NULL AND revoked_at IS NULL AND expires_at>now())`, c.ID, email).Scan(&pending); err != nil {
		dbError(w, err)
		return
	}
	if pending {
		fail(w, 409, "invitation_pending", "A pending invitation already exists; revoke it before issuing another")
		return
	}
	raw, hash, err := token()
	if err != nil {
		dbError(w, err)
		return
	}
	i := Invitation{ID: newID(), Email: email, Role: in.Role, ExpiresAt: time.Now().UTC().Add(7 * 24 * time.Hour)}
	if _, err = tx.Exec(r.Context(), `INSERT INTO club_invitations(id,club_id,email,role,token_hash,created_by,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, i.ID, c.ID, email, in.Role, hash, u.ID, i.ExpiresAt); err != nil {
		dbError(w, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		dbError(w, err)
		return
	}
	// Raw invitation tokens leave the process only in the recipient's email.
	body := "Foi convidado para o clube " + c.Name + " na FUTRYVA.\n\nCódigo do convite: " + raw + "\n\nEntre na sua conta com este email e introduza o código em Aceitar convite. O convite expira em 7 dias."
	if err = s.mailer.Send(r.Context(), email, "Convite FUTRYVA", body); err != nil {
		cleanup, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, _ = s.db.Exec(cleanup, `UPDATE club_invitations SET revoked_at=now() WHERE id=$1 AND accepted_at IS NULL`, i.ID)
		fail(w, 503, "email_unavailable", "Unable to send invitation; please retry")
		return
	}
	jsonResponse(w, 201, map[string]any{"invitation": i})
}
func (s *Service) revokeInvitation(w http.ResponseWriter, r *http.Request, u identity.User) {
	tx, c, ok := s.transaction(w, r, u)
	if !ok {
		return
	}
	defer tx.Rollback(context.Background())
	id := r.PathValue("invitationId")
	if !validID(id) {
		fail(w, 404, "not_found", "Invitation not found")
		return
	}
	var role string
	if err := tx.QueryRow(r.Context(), `SELECT role FROM club_invitations WHERE id=$1 AND club_id=$2 AND accepted_at IS NULL FOR UPDATE`, id, c.ID).Scan(&role); err != nil {
		dbError(w, err)
		return
	}
	if role == "director" && c.Role != "president" {
		fail(w, 403, "forbidden", "Only the president may revoke a director invitation")
		return
	}
	if _, err := tx.Exec(r.Context(), `UPDATE club_invitations SET revoked_at=COALESCE(revoked_at,now()) WHERE id=$1 AND club_id=$2`, id, c.ID); err != nil {
		dbError(w, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		dbError(w, err)
		return
	}
	jsonResponse(w, 204, nil)
}
func (s *Service) acceptInvitation(w http.ResponseWriter, r *http.Request, u identity.User) {
	if !verified(w, u) {
		return
	}
	var in struct {
		Token string `json:"token"`
	}
	if !decode(w, r, &in) {
		return
	}
	if len(in.Token) != 43 || strings.ContainsAny(in.Token, " \r\n") {
		invalidInvitation(w)
		return
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		dbError(w, err)
		return
	}
	defer tx.Rollback(context.Background())
	hash := hashToken(in.Token)
	var clubID string
	// Lock clubs before invitations, matching the order used by create/revoke.
	err = tx.QueryRow(r.Context(), `SELECT c.id::text FROM clubs c WHERE c.archived_at IS NULL AND c.id=(SELECT club_id FROM club_invitations WHERE token_hash=$1) FOR UPDATE`, hash).Scan(&clubID)
	if errors.Is(err, pgx.ErrNoRows) {
		invalidInvitation(w)
		return
	}
	if err != nil {
		dbError(w, err)
		return
	}
	var id, role string
	err = tx.QueryRow(r.Context(), `SELECT id::text,role FROM club_invitations WHERE token_hash=$1 AND club_id=$2 AND lower(email)=$3 AND accepted_at IS NULL AND revoked_at IS NULL AND expires_at>now() FOR UPDATE`, hash, clubID, strings.ToLower(u.Email)).Scan(&id, &role)
	if errors.Is(err, pgx.ErrNoRows) {
		invalidInvitation(w)
		return
	}
	if err != nil {
		dbError(w, err)
		return
	}
	// Assignment is singular and read under a user lock, after the club lock.
	if err = assignUser(r.Context(), tx, clubID, u.ID, false); err != nil {
		assignmentError(w, err)
		return
	}
	if _, err = tx.Exec(r.Context(), `INSERT INTO club_memberships(club_id,user_id,role) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, clubID, u.ID, role); err != nil {
		dbError(w, err)
		return
	}
	if _, err = tx.Exec(r.Context(), `UPDATE club_invitations SET accepted_at=now() WHERE id=$1`, id); err != nil {
		dbError(w, err)
		return
	}
	c, err := clubFor(r.Context(), tx, u.ID, clubID, false)
	if err != nil {
		dbError(w, err)
		return
	}
	if _, err = tx.Exec(r.Context(), `INSERT INTO platform_events(actor_id,action,club_id,target_user_id) VALUES($1,'invitation_accepted',$2,$1)`, u.ID, clubID); err != nil {
		dbError(w, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		dbError(w, err)
		return
	}
	jsonResponse(w, 200, map[string]any{"club": c})
}
func invalidInvitation(w http.ResponseWriter) {
	fail(w, 400, "invalid_invitation", "Invitation is invalid, expired or does not belong to this email")
}
