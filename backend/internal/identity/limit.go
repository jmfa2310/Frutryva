package identity

import (
	"net"
	"net/http"
	"sync"
	"time"
)

const authRequestsPerMinute = 60
const maximumTrackedPeers = 4096

type rateWindow struct {
	since time.Time
	count int
}
type rateLimiter struct {
	mu        sync.Mutex
	entries   map[string]rateWindow
	now       func() time.Time
	nextSweep time.Time
}

func newRateLimiter() *rateLimiter {
	return &rateLimiter{entries: map[string]rateWindow{}, now: time.Now}
}

// This bounded, single-instance limiter keys only by the actual transport peer.
// Arbitrary X-Forwarded-For headers must never bypass authentication limits. A
// reverse proxy consequently shares a bucket; production requires a deliberate
// trusted-proxy/distributed-limiter configuration rather than trusting headers.
func (l *rateLimiter) allow(remote string) bool {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		host = remote
	}
	ip := net.ParseIP(host)
	if ip == nil {
		host = "unknown"
	} else {
		host = ip.String()
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if !now.Before(l.nextSweep) {
		for peer, entry := range l.entries {
			if now.Sub(entry.since) >= time.Minute {
				delete(l.entries, peer)
			}
		}
		l.nextSweep = now.Add(time.Minute)
	}
	entry, exists := l.entries[host]
	if !exists {
		if len(l.entries) >= maximumTrackedPeers {
			return false
		}
		entry = rateWindow{since: now}
	} else if now.Sub(entry.since) >= time.Minute {
		entry = rateWindow{since: now}
	}
	if entry.count >= authRequestsPerMinute {
		return false
	}
	entry.count++
	l.entries[host] = entry
	return true
}

func (s *Service) limited(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.limiter.allow(r.RemoteAddr) {
			w.Header().Set("Retry-After", "60")
			writeError(w, 429, "rate_limited", "Demasiados pedidos de acesso. Aguarde um minuto e tente novamente.")
			return
		}
		next(w, r)
	}
}
