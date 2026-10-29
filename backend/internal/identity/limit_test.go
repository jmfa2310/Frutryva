package identity

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestAuthenticationLimitCannotBeBypassedByForwardedHeaders(t *testing.T) {
	service := New(nil, nil, false)
	mux := http.NewServeMux()
	service.Mount(mux)
	for i := 0; i <= authRequestsPerMinute; i++ {
		r := httptest.NewRequest("POST", "/v1/auth/forgot-password", strings.NewReader(`{"email":"invalid"}`))
		r.RemoteAddr = "192.0.2.1:" + strconv.Itoa(10000+i)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-Forwarded-For", "198.51.100."+strconv.Itoa(i))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if i < authRequestsPerMinute {
			status(t, w, 202)
		} else {
			status(t, w, 429)
			if w.Header().Get("Retry-After") != "60" {
				t.Fatal("rate limit missing retry guidance")
			}
		}
	}
}

func TestAuthenticationLimitExpiresAndBoundsTrackedPeers(t *testing.T) {
	clock := time.Now()
	limiter := newRateLimiter()
	limiter.now = func() time.Time { return clock }
	for i := 0; i < authRequestsPerMinute; i++ {
		if !limiter.allow("192.0.2.1:1000") {
			t.Fatal("limit applied early")
		}
	}
	if limiter.allow("192.0.2.1:2000") {
		t.Fatal("changing source port bypassed limit")
	}
	for i := 0; i < maximumTrackedPeers-1; i++ {
		peer := "[2001:db8::" + strconv.FormatInt(int64(i+1), 16) + "]:1000"
		if !limiter.allow(peer) {
			t.Fatal("peer cap applied early")
		}
	}
	if limiter.allow("198.51.100.1:1000") || len(limiter.entries) != maximumTrackedPeers {
		t.Fatal("limiter map exceeded capacity")
	}
	clock = clock.Add(time.Minute + time.Second)
	if !limiter.allow("198.51.100.1:1000") || len(limiter.entries) != 1 {
		t.Fatal("expired entries were not reclaimed")
	}
}
