package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type fakeStore struct {
	readyErr  error
	sportsErr error
	variants  []Variant
	deadline  time.Time
	calls     int
}

func (s *fakeStore) Ready(ctx context.Context) error {
	s.deadline, _ = ctx.Deadline()
	s.calls++
	return s.readyErr
}
func (s *fakeStore) Sports(ctx context.Context) ([]Variant, error) {
	s.deadline, _ = ctx.Deadline()
	s.calls++
	return s.variants, s.sportsErr
}
func testHandler(s Store) http.Handler { return New(s, slog.New(slog.NewTextHandler(io.Discard, nil))) }

func TestHealthChecksReadinessWithDeadline(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		status int
	}{
		{"healthy", nil, 200}, {"unavailable", errors.New("password=do-not-leak"), 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &fakeStore{readyErr: tc.err}
			rec := httptest.NewRecorder()
			testHandler(s).ServeHTTP(rec, httptest.NewRequest("GET", "/health", nil))
			if rec.Code != tc.status {
				t.Fatalf("status %d", rec.Code)
			}
			if s.deadline.IsZero() || time.Until(s.deadline) > 3*time.Second {
				t.Fatal("missing readiness timeout")
			}
			if strings.Contains(rec.Body.String(), "do-not-leak") {
				t.Fatal("database error leaked")
			}
			var payload map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatal(err)
			}
			if tc.err == nil && (payload["status"] != "ok" || payload["service"] != "FUTRYVA API") {
				t.Fatal("existing response shape changed")
			}
		})
	}
}

func TestSportsPreservesContractAndEmptyArray(t *testing.T) {
	for _, tc := range []struct {
		name     string
		variants []Variant
		want     string
	}{
		{"seeded", []Variant{{SportCode: "futsal", Sport: "Futsal", Variant: "Futsal", Players: 5}}, `[{"sportCode":"futsal","sport":"Futsal","variant":"Futsal","playersOnField":5}]`},
		{"empty", nil, `[]`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &fakeStore{variants: tc.variants}
			rec := httptest.NewRecorder()
			testHandler(s).ServeHTTP(rec, httptest.NewRequest("GET", "/v1/sports", nil))
			if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != tc.want {
				t.Fatalf("unexpected response: %d %s", rec.Code, rec.Body.String())
			}
			if s.deadline.IsZero() || time.Until(s.deadline) > 5*time.Second {
				t.Fatal("missing catalog timeout")
			}
		})
	}
}

func TestSportsFailureIsStructuredAndRedacted(t *testing.T) {
	rec := httptest.NewRecorder()
	testHandler(&fakeStore{sportsErr: errors.New("postgres://secret@host")}).ServeHTTP(rec, httptest.NewRequest("GET", "/v1/sports", nil))
	if rec.Code != 500 || strings.Contains(rec.Body.String(), "secret") {
		t.Fatalf("unexpected error response %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"code":"database_error"`) {
		t.Fatal("missing stable error code")
	}
}

func TestRoutesRejectUnknownPathsAndUnsupportedMethods(t *testing.T) {
	for _, tc := range []struct {
		method, path string
		status       int
	}{
		{"GET", "/", 200}, {"GET", "/v1/unknown", 404}, {"GET", "/health/extra", 404},
		{"GET", "/v1/sports/", 404}, {"POST", "/health", 405}, {"DELETE", "/v1/sports", 405}, {"POST", "/", 405},
	} {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			s := &fakeStore{}
			rec := httptest.NewRecorder()
			testHandler(s).ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
			if rec.Code != tc.status {
				t.Fatalf("expected %d got %d", tc.status, rec.Code)
			}
			if !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
				t.Fatal("response should be JSON")
			}
			if tc.status == 405 && rec.Header().Get("Allow") != "GET, HEAD" {
				t.Fatal("missing Allow header")
			}
			if s.calls != 0 {
				t.Fatal("invalid requests must not access the database")
			}
		})
	}
}
