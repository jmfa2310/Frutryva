package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestContainerHealthcheckRequiresReadyResponse(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusServiceUnavailable} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var called atomic.Bool
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called.Store(true)
				if r.URL.Path != "/health" {
					t.Errorf("unexpected health path %s", r.URL.Path)
				}
				w.WriteHeader(status)
			}))
			defer server.Close()
			_, port, err := net.SplitHostPort(server.Listener.Addr().String())
			if err != nil {
				t.Fatal(err)
			}
			t.Setenv("PORT", port)
			err = healthcheck()
			if !called.Load() || (err == nil) != (status == http.StatusOK) {
				t.Fatalf("called=%v healthcheck error=%v", called.Load(), err)
			}
		})
	}
}
