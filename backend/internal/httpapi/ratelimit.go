package httpapi

import (
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// authFlowLimiter throttles per-IP hits against auth-adjacent endpoints
// that trigger an outbound redirect to Authentik (currently just
// /auth/reauth). Self-hosted single-family app, so a simple in-memory
// per-IP bucket is enough — no need for a shared store.
type authFlowLimiter struct {
	mu       sync.Mutex
	limiters map[string]*visitor
}

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func newAuthFlowLimiter() *authFlowLimiter {
	return &authFlowLimiter{limiters: make(map[string]*visitor)}
}

func (rl *authFlowLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Opportunistic cleanup so the map doesn't grow unbounded over uptime.
	if len(rl.limiters) > 1000 {
		for k, v := range rl.limiters {
			if time.Since(v.lastSeen) > 10*time.Minute {
				delete(rl.limiters, k)
			}
		}
	}

	v, ok := rl.limiters[key]
	if !ok {
		// 10 attempts per minute, burst 10 — this endpoint is already
		// requireAuth-gated and each hit just triggers an Authentik redirect,
		// so this is light hygiene throttling, not a brute-force defense.
		v = &visitor{limiter: rate.NewLimiter(rate.Every(6*time.Second), 10)}
		rl.limiters[key] = v
	}
	v.lastSeen = time.Now()
	return v.limiter.Allow()
}

func (rl *authFlowLimiter) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}
		if !rl.allow(host) {
			Error(w, http.StatusTooManyRequests, "rate_limited", "too many attempts, try again later")
			return
		}
		next.ServeHTTP(w, r)
	})
}
