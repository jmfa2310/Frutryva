package httpapi

import (
	"context"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Secure enforces same-origin writes for browser cookie sessions. API clients
// without browser Origin headers can use JSON; no cross-origin CORS is enabled.
func Secure(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
		defer cancel()
		r = r.WithContext(ctx)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		defer func() {
			if recover() != nil {
				logger.Error("request failed unexpectedly")
				writeError(w, 500, "internal_error", "Ocorreu um erro inesperado")
			}
		}()
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
			origin := r.Header.Get("Origin")
			if origin != "" {
				u, err := url.Parse(origin)
				if err != nil || (u.Scheme != "http" && u.Scheme != "https") || !strings.EqualFold(u.Host, r.Host) || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" {
					writeError(w, 403, "origin_not_allowed", "Origem do pedido não permitida")
					return
				}
			}
			if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
				writeError(w, 403, "origin_not_allowed", "Origem do pedido não permitida")
				return
			}
			if r.Method != http.MethodDelete {
				kind, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
				if err != nil || kind != "application/json" {
					writeError(w, 415, "json_required", "Envie os dados em JSON")
					return
				}
			}
			r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		}
		next.ServeHTTP(w, r)
	})
}
