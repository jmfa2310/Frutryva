// Package identity manages cookie sessions and one-use account recovery tokens.
package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

const CookieName = "futryva_session"
const sessionLifetime = 24 * time.Hour
const passwordCost = 12

var (
	ErrUnauthenticated = errors.New("authentication required")
	ErrInvalidInput    = errors.New("invalid account details")
	ErrConflict        = errors.New("email already registered")
	ErrInvalidToken    = errors.New("invalid or expired token")
)

// A fixed valid bcrypt hash makes unknown-account login perform password work.
var dummyHash = []byte("$2a$12$R9h/cIPz0gi.URNNX3kh2OPST9/PgBkqquzi.Ss7KIUgO2t0jWMUW")

type User struct {
	ID              string `json:"id"`
	Email           string `json:"email"`
	DisplayName     string `json:"displayName"`
	EmailVerified   bool   `json:"emailVerified"`
	IsPlatformOwner bool   `json:"isPlatformOwner"`
	AssignedClubID  string `json:"assignedClubId,omitempty"`
}

type Mailer interface {
	Send(context.Context, string, string, string) error
}

type Service struct {
	db           *pgxpool.Pool
	mailer       Mailer
	secureCookie bool
	limiter      *rateLimiter
}

func New(db *pgxpool.Pool, mailer Mailer, secureCookie bool) *Service {
	return &Service{db: db, mailer: mailer, secureCookie: secureCookie, limiter: newRateLimiter()}
}

func randomToken() (string, error) {
	var data [32]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data[:]), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func validToken(token string) bool {
	b, err := base64.RawURLEncoding.DecodeString(token)
	return err == nil && len(b) == 32
}

func randomID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}

func normalizedEmail(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	address, err := mail.ParseAddress(value)
	if err != nil || address.Address != value || len(value) > 254 || strings.ContainsAny(value, "\r\n") {
		return "", ErrInvalidInput
	}
	return value, nil
}

func validPassword(value string) bool { return len(value) >= 12 && len(value) <= 72 }

func (s *Service) CurrentUser(ctx context.Context, r *http.Request) (User, error) {
	cookie, err := r.Cookie(CookieName)
	if err != nil || !validToken(cookie.Value) {
		return User{}, ErrUnauthenticated
	}
	var user User
	err = s.db.QueryRow(ctx, `SELECT u.id,u.email,u.display_name,u.email_verified_at IS NOT NULL,
		COALESCE(u.assigned_club_id::text,''),EXISTS(SELECT 1 FROM platform_owners o WHERE o.user_id=u.id)
		FROM auth_sessions s JOIN users u ON u.id=s.user_id
		WHERE s.token_hash=$1 AND s.revoked_at IS NULL AND s.expires_at>now()`, hashToken(cookie.Value)).Scan(&user.ID, &user.Email, &user.DisplayName, &user.EmailVerified, &user.AssignedClubID, &user.IsPlatformOwner)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrUnauthenticated
	}
	return user, err
}

func (s *Service) Require(next func(http.ResponseWriter, *http.Request, User)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := s.CurrentUser(r.Context(), r)
		if err != nil {
			if errors.Is(err, ErrUnauthenticated) {
				writeError(w, 401, "unauthenticated", "Inicie sessão para continuar.")
			} else {
				writeError(w, 503, "database_unavailable", "Não foi possível validar a sessão.")
			}
			return
		}
		next(w, r, user)
	}
}

func (s *Service) setCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{Name: CookieName, Value: token, Path: "/", HttpOnly: true, Secure: s.secureCookie, SameSite: http.SameSiteLaxMode, MaxAge: int(sessionLifetime.Seconds()), Expires: time.Now().Add(sessionLifetime)})
}

func (s *Service) clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: CookieName, Value: "", Path: "/", HttpOnly: true, Secure: s.secureCookie, SameSite: http.SameSiteLaxMode, MaxAge: -1, Expires: time.Unix(1, 0)})
}

func insertSession(ctx context.Context, tx pgx.Tx, userID, token string) error {
	_, err := tx.Exec(ctx, `INSERT INTO auth_sessions(token_hash,user_id,expires_at) VALUES($1,$2,now()+interval '24 hours')`, hashToken(token), userID)
	return err
}

func insertActionToken(ctx context.Context, tx pgx.Tx, userID, purpose, token string) error {
	expires := time.Now().Add(24 * time.Hour)
	if purpose == "reset_password" {
		expires = time.Now().Add(30 * time.Minute)
	}
	_, err := tx.Exec(ctx, `INSERT INTO auth_tokens(token_hash,user_id,purpose,expires_at) VALUES($1,$2,$3,$4)`, hashToken(token), userID, purpose, expires)
	return err
}

func (s *Service) sendAction(ctx context.Context, user User, purpose, token string) error {
	if s.mailer == nil {
		return errors.New("email delivery is unavailable")
	}
	subject := "FUTRYVA — confirmar email"
	body := "Para confirmar o seu email, introduza este código na FUTRYVA:\r\n\r\n" + token + "\r\n\r\nO código é de uso único e expira em 24 horas.\r\n"
	if purpose == "reset_password" {
		subject = "FUTRYVA — recuperar acesso"
		body = "Para definir uma nova palavra-passe, introduza este código na FUTRYVA:\r\n\r\n" + token + "\r\n\r\nO código é de uso único e expira em 30 minutos. Se não pediu esta alteração, ignore a mensagem.\r\n"
	}
	return s.mailer.Send(ctx, user.Email, subject, body)
}

func (s *Service) register(ctx context.Context, email, name, password string) (User, string, bool, error) {
	email, err := normalizedEmail(email)
	name = strings.TrimSpace(name)
	if err != nil || !validPassword(password) || utf8.RuneCountInString(name) < 2 || utf8.RuneCountInString(name) > 100 {
		return User{}, "", false, ErrInvalidInput
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), passwordCost)
	if err != nil {
		return User{}, "", false, err
	}
	id, err := randomID()
	if err != nil {
		return User{}, "", false, err
	}
	session, err := randomToken()
	if err != nil {
		return User{}, "", false, err
	}
	action, err := randomToken()
	if err != nil {
		return User{}, "", false, err
	}
	user := User{ID: id, Email: email, DisplayName: name}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return User{}, "", false, err
	}
	defer tx.Rollback(context.Background())
	_, err = tx.Exec(ctx, `INSERT INTO users(id,email,display_name,password_hash) VALUES($1,$2,$3,$4)`, id, email, name, string(hash))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return User{}, "", false, ErrConflict
		}
		return User{}, "", false, err
	}
	if err = insertSession(ctx, tx, id, session); err != nil {
		return User{}, "", false, err
	}
	if err = insertActionToken(ctx, tx, id, "verify_email", action); err != nil {
		return User{}, "", false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return User{}, "", false, err
	}
	pending := s.sendAction(ctx, user, "verify_email", action) != nil
	return user, session, pending, nil
}

func (s *Service) login(ctx context.Context, email, password string) (User, string, error) {
	email, err := normalizedEmail(email)
	if err != nil || !validPassword(password) {
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte("invalid-password"))
		return User{}, "", ErrUnauthenticated
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return User{}, "", err
	}
	defer tx.Rollback(context.Background())
	var user User
	var hash *string
	err = tx.QueryRow(ctx, `SELECT u.id,u.email,u.display_name,u.email_verified_at IS NOT NULL,u.password_hash,
		COALESCE(u.assigned_club_id::text,''),EXISTS(SELECT 1 FROM platform_owners o WHERE o.user_id=u.id)
		FROM users u WHERE lower(u.email)=$1 FOR UPDATE OF u`, email).Scan(&user.ID, &user.Email, &user.DisplayName, &user.EmailVerified, &hash, &user.AssignedClubID, &user.IsPlatformOwner)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && hash == nil {
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
		return User{}, "", ErrUnauthenticated
	}
	if err != nil {
		return User{}, "", err
	}
	if bcrypt.CompareHashAndPassword([]byte(*hash), []byte(password)) != nil {
		return User{}, "", ErrUnauthenticated
	}
	token, err := randomToken()
	if err != nil {
		return User{}, "", err
	}
	if err = insertSession(ctx, tx, user.ID, token); err != nil {
		return User{}, "", err
	}
	if err = tx.Commit(ctx); err != nil {
		return User{}, "", err
	}
	return user, token, nil
}

func (s *Service) revoke(ctx context.Context, token string) error {
	if !validToken(token) {
		return nil
	}
	_, err := s.db.Exec(ctx, `UPDATE auth_sessions SET revoked_at=now() WHERE token_hash=$1 AND revoked_at IS NULL`, hashToken(token))
	return err
}

func (s *Service) refresh(ctx context.Context, raw string) (User, string, error) {
	if !validToken(raw) {
		return User{}, "", ErrUnauthenticated
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return User{}, "", err
	}
	defer tx.Rollback(context.Background())
	var user User
	// Lock the user before sessions, just as login/reset do. This prevents a
	// refresh racing a password reset from leaving a new session unrevoked.
	err = tx.QueryRow(ctx, `SELECT u.id,u.email,u.display_name,u.email_verified_at IS NOT NULL,
		COALESCE(u.assigned_club_id::text,''),EXISTS(SELECT 1 FROM platform_owners o WHERE o.user_id=u.id) FROM users u
		WHERE u.id=(SELECT user_id FROM auth_sessions WHERE token_hash=$1) FOR UPDATE OF u`, hashToken(raw)).Scan(&user.ID, &user.Email, &user.DisplayName, &user.EmailVerified, &user.AssignedClubID, &user.IsPlatformOwner)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, "", ErrUnauthenticated
	}
	if err != nil {
		return User{}, "", err
	}
	tag, err := tx.Exec(ctx, `UPDATE auth_sessions SET revoked_at=now() WHERE token_hash=$1 AND revoked_at IS NULL AND expires_at>now()`, hashToken(raw))
	if err != nil {
		return User{}, "", err
	}
	if tag.RowsAffected() != 1 {
		return User{}, "", ErrUnauthenticated
	}
	token, err := randomToken()
	if err != nil {
		return User{}, "", err
	}
	if err = insertSession(ctx, tx, user.ID, token); err != nil {
		return User{}, "", err
	}
	if err = tx.Commit(ctx); err != nil {
		return User{}, "", err
	}
	return user, token, nil
}

func (s *Service) requestAction(ctx context.Context, email, purpose string) error {
	user := User{}
	err := s.db.QueryRow(ctx, `SELECT id,email,display_name,email_verified_at IS NOT NULL FROM users WHERE lower(email)=$1`, email).Scan(&user.ID, &user.Email, &user.DisplayName, &user.EmailVerified)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if purpose == "verify_email" && user.EmailVerified {
		return nil
	}
	token, err := randomToken()
	if err != nil {
		return err
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if err = insertActionToken(ctx, tx, user.ID, purpose, token); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	return s.sendAction(ctx, user, purpose, token)
}

func (s *Service) consumeAction(ctx context.Context, raw, purpose, password string) error {
	if !validToken(raw) {
		return ErrInvalidToken
	}
	var newHash []byte
	var err error
	if purpose == "reset_password" {
		if !validPassword(password) {
			return ErrInvalidInput
		}
		newHash, err = bcrypt.GenerateFromPassword([]byte(password), passwordCost)
		if err != nil {
			return err
		}
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	var userID string
	err = tx.QueryRow(ctx, `SELECT user_id FROM auth_tokens WHERE token_hash=$1 AND purpose=$2 AND used_at IS NULL AND expires_at>now()`, hashToken(raw), purpose).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrInvalidToken
	}
	if err != nil {
		return err
	}
	// Account operations always acquire the user lock before token/session
	// locks. Recheck the token afterwards so simultaneous consumes cannot race.
	if err = tx.QueryRow(ctx, `SELECT id FROM users WHERE id=$1 FOR UPDATE`, userID).Scan(&userID); err != nil {
		return err
	}
	err = tx.QueryRow(ctx, `SELECT user_id FROM auth_tokens WHERE token_hash=$1 AND purpose=$2 AND used_at IS NULL AND expires_at>now() FOR UPDATE`, hashToken(raw), purpose).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrInvalidToken
	}
	if err != nil {
		return err
	}
	if purpose == "verify_email" {
		_, err = tx.Exec(ctx, `UPDATE users SET email_verified_at=COALESCE(email_verified_at,now()) WHERE id=$1`, userID)
	} else {
		_, err = tx.Exec(ctx, `UPDATE users SET password_hash=$2 WHERE id=$1`, userID, string(newHash))
	}
	if err != nil {
		return err
	}
	if purpose == "reset_password" {
		if _, err = tx.Exec(ctx, `UPDATE auth_sessions SET revoked_at=now() WHERE user_id=$1 AND revoked_at IS NULL`, userID); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE auth_tokens SET used_at=now() WHERE user_id=$1 AND purpose=$2 AND used_at IS NULL`, userID, purpose); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
