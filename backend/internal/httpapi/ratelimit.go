package httpapi

import (
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// passwordRateLimiter throttles per-IP attempts against password-protected
// endpoints (year lock/unlock, period delete) so a brute-force script can't
// hammer DELETE_PASSWORD. Self-hosted single-family app, so a simple
// in-memory per-IP bucket is enough — no need for a shared store.
type passwordRateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*visitor
}

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func newPasswordRateLimiter() *passwordRateLimiter {
	return &passwordRateLimiter{limiters: make(map[string]*visitor)}
}

func (rl *passwordRateLimiter) allow(key string) bool {
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
		// 5 attempts per minute, burst 5 — enough for a mistyped password
		// retried a couple of times, not enough for a brute-force script.
		v = &visitor{limiter: rate.NewLimiter(rate.Every(12*time.Second), 5)}
		rl.limiters[key] = v
	}
	v.lastSeen = time.Now()
	return v.limiter.Allow()
}

func (rl *passwordRateLimiter) middleware(next http.Handler) http.Handler {
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
