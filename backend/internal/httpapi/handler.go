package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

func New(store Store, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	Mount(mux, store, logger)
	return mux
}

// Mount keeps the foundation endpoints available alongside the domain modules.
func Mount(mux *http.ServeMux, store Store, logger *slog.Logger) {
	mux.HandleFunc("/health", getOnly(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		if err := store.Ready(ctx); err != nil {
			logger.Warn("readiness check failed")
			writeError(w, http.StatusServiceUnavailable, "database_unavailable", "Database or schema unavailable")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "FUTRYVA API"})
	}))
	mux.HandleFunc("/v1/sports", getOnly(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		variants, err := store.Sports(ctx)
		if err != nil {
			logger.Error("sports catalog query failed")
			writeError(w, http.StatusInternalServerError, "database_error", "Unable to load sports")
			return
		}
		if variants == nil {
			variants = []Variant{}
		}
		writeJSON(w, http.StatusOK, variants)
	}))
	mux.HandleFunc("/{$}", getOnly(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"name": "FUTRYVA", "stage": "initial foundation", "sports": "GET /v1/sports", "health": "GET /health"})
	}))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "Resource not found")
	})
}

func getOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
			return
		}
		next(w, r)
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
