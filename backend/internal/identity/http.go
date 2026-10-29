package identity

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"

	"github.com/jackc/pgx/v5"
)

func (s *Service) Mount(mux *http.ServeMux) {
	mux.HandleFunc("/v1/auth/register", method(http.MethodPost, s.limited(s.registerHTTP)))
	mux.HandleFunc("/v1/auth/login", method(http.MethodPost, s.limited(s.loginHTTP)))
	mux.HandleFunc("/v1/auth/logout", method(http.MethodPost, s.limited(s.logoutHTTP)))
	mux.HandleFunc("/v1/auth/refresh", method(http.MethodPost, s.limited(s.refreshHTTP)))
	mux.HandleFunc("/v1/auth/verify-email", method(http.MethodPost, s.limited(s.verifyHTTP)))
	mux.HandleFunc("/v1/auth/resend-verification", method(http.MethodPost, s.limited(s.Require(s.resendHTTP))))
	mux.HandleFunc("/v1/auth/forgot-password", method(http.MethodPost, s.limited(s.forgotHTTP)))
	mux.HandleFunc("/v1/auth/reset-password", method(http.MethodPost, s.limited(s.resetHTTP)))
	mux.HandleFunc("/v1/me", method(http.MethodGet, s.Require(s.meHTTP)))
}

func method(allowed string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != allowed && !(allowed == http.MethodGet && r.Method == http.MethodHead) {
			w.Header().Set("Allow", allowed)
			writeError(w, 405, "method_not_allowed", "Método não permitido.")
			return
		}
		next(w, r)
	}
}

func decode(w http.ResponseWriter, r *http.Request, value any) bool {
	contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || contentType != "application/json" {
		writeError(w, 415, "unsupported_media_type", "Envie os dados em JSON.")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		writeError(w, 400, "invalid_json", "Dados JSON inválidos.")
		return false
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		writeError(w, 400, "invalid_json", "Envie apenas um objeto JSON.")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	if status != http.StatusNoContent {
		_ = json.NewEncoder(w).Encode(value)
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func accountError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput):
		writeError(w, 400, "invalid_input", "Verifique o email, nome e palavra-passe (12 a 72 bytes).")
	case errors.Is(err, ErrConflict):
		writeError(w, 409, "email_in_use", "Já existe uma conta com este email.")
	case errors.Is(err, ErrUnauthenticated), errors.Is(err, pgx.ErrNoRows):
		writeError(w, 401, "invalid_credentials", "Email ou palavra-passe incorretos, ou sessão expirada.")
	case errors.Is(err, ErrInvalidToken):
		writeError(w, 400, "invalid_token", "O código é inválido, já foi utilizado ou expirou.")
	default:
		writeError(w, 503, "service_unavailable", "O serviço está temporariamente indisponível.")
	}
}

func (s *Service) registerHTTP(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email       string `json:"email"`
		DisplayName string `json:"displayName"`
		Password    string `json:"password"`
	}
	if !decode(w, r, &input) {
		return
	}
	user, session, pending, err := s.register(r.Context(), input.Email, input.DisplayName, input.Password)
	if err != nil {
		accountError(w, err)
		return
	}
	s.setCookie(w, session)
	writeJSON(w, 201, struct {
		User    User `json:"user"`
		Pending bool `json:"emailDeliveryPending,omitempty"`
	}{user, pending})
}

func (s *Service) loginHTTP(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decode(w, r, &input) {
		return
	}
	user, token, err := s.login(r.Context(), input.Email, input.Password)
	if err != nil {
		accountError(w, err)
		return
	}
	s.setCookie(w, token)
	writeJSON(w, 200, map[string]any{"user": user})
}

func (s *Service) logoutHTTP(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(CookieName); err == nil {
		if err := s.revoke(r.Context(), cookie.Value); err != nil {
			accountError(w, err)
			return
		}
	}
	s.clearCookie(w)
	writeJSON(w, 204, nil)
}

func (s *Service) refreshHTTP(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(CookieName)
	if err != nil {
		accountError(w, ErrUnauthenticated)
		return
	}
	user, token, err := s.refresh(r.Context(), cookie.Value)
	if err != nil {
		accountError(w, err)
		return
	}
	s.setCookie(w, token)
	writeJSON(w, 200, map[string]any{"user": user})
}

func (s *Service) meHTTP(w http.ResponseWriter, r *http.Request, user User) {
	writeJSON(w, 200, map[string]any{"user": user})
}

func (s *Service) verifyHTTP(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Token string `json:"token"`
	}
	if !decode(w, r, &input) {
		return
	}
	if err := s.consumeAction(r.Context(), input.Token, "verify_email", ""); err != nil {
		accountError(w, err)
		return
	}
	writeJSON(w, 204, nil)
}

func (s *Service) resendHTTP(w http.ResponseWriter, r *http.Request, user User) {
	if err := s.requestAction(r.Context(), user.Email, "verify_email"); err != nil {
		accountError(w, err)
		return
	}
	writeJSON(w, 202, map[string]string{"message": "Se necessário, foi enviado um novo código de confirmação."})
}

func (s *Service) forgotHTTP(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email string `json:"email"`
	}
	if !decode(w, r, &input) {
		return
	}
	if email, err := normalizedEmail(input.Email); err == nil {
		// The public response deliberately does not disclose account existence,
		// verification state, or SMTP delivery details.
		_ = s.requestAction(r.Context(), email, "reset_password")
	}
	writeJSON(w, 202, map[string]string{"message": "Se a conta existir, receberá um código para recuperar o acesso."})
}

func (s *Service) resetHTTP(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if !decode(w, r, &input) {
		return
	}
	if err := s.consumeAction(r.Context(), input.Token, "reset_password", input.Password); err != nil {
		accountError(w, err)
		return
	}
	s.clearCookie(w)
	writeJSON(w, 204, nil)
}
