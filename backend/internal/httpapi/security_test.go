package httpapi

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSameOriginWritesAndJSON(t *testing.T) {
	for _, tc := range []struct {
		name, origin, site, content, method string
		want                                int
	}{
		{"same origin", "http://example.com", "same-origin", "application/json", "POST", 204},
		{"JSON CLI", "", "", "application/json", "POST", 204},
		{"cross origin", "http://attacker.invalid", "", "application/json", "POST", 403},
		{"null origin", "null", "", "application/json", "POST", 403},
		{"cross site", "", "cross-site", "application/json", "POST", 403},
		{"form rejected", "", "", "application/x-www-form-urlencoded", "POST", 415},
		{"text rejected", "", "", "text/plain", "POST", 415},
		{"read allowed", "", "", "", "GET", 204},
		{"delete allowed", "http://example.com", "same-origin", "", "DELETE", 204},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, "http://example.com/v1/clubs", nil)
			r.Header.Set("Origin", tc.origin)
			r.Header.Set("Sec-Fetch-Site", tc.site)
			r.Header.Set("Content-Type", tc.content)
			w := httptest.NewRecorder()
			Secure(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }), slog.New(slog.NewTextHandler(io.Discard, nil))).ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Fatalf("status=%d want=%d", w.Code, tc.want)
			}
			if w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("private response must not be cached")
			}
		})
	}
}
