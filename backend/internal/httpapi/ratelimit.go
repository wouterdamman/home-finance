package httpapi

import (
	"container/list"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/wouterdamman/home-finance/internal/auth"
)

// authFlowLimiter throttles hits against auth-adjacent endpoints that trigger
// an outbound redirect to Authentik (currently just /auth/reauth). Self-hosted
// single-family app, so a simple in-memory bucket is enough — no shared store.
//
// Buckets are keyed on the authenticated session user where there is one.
// r.RemoteAddr is rewritten by middleware.RealIP from X-Forwarded-For /
// X-Real-IP / True-Client-IP with no trusted-proxy list (see GHSA-3fxj-6jh8-hvhx),
// so it is caller-controlled: rotating the header both evades the limit and
// grows the bucket map unboundedly. It stays as the fallback for genuinely
// unauthenticated routes, where there is nothing better — bounded by maxEntries.
type authFlowLimiter struct {
	mu       sync.Mutex
	limiters map[string]*list.Element
	// order is least-recently-used first, so eviction is O(1) instead of the
	// full-map scan the previous age-only sweep did under this mutex on every
	// single request.
	order *list.List
}

// maxEntries hard-caps the bucket map. Reached only under an attack or a
// misconfigured proxy; a legitimate deployment holds a handful of entries.
const maxEntries = 4096

// visitorTTL is how long an idle bucket is kept. Longer than the bucket's own
// refill window, so dropping it can never hand a caller a fresh burst.
const visitorTTL = 10 * time.Minute

// limiterSweepInterval is how often idle buckets are reclaimed in the
// background. Same lifetime-of-the-process pattern as the session store's
// cleanup goroutine (auth/session.go).
const limiterSweepInterval = 5 * time.Minute

type visitor struct {
	key      string
	limiter  *rate.Limiter
	lastSeen time.Time
}

func newAuthFlowLimiter() *authFlowLimiter {
	rl := &authFlowLimiter{limiters: make(map[string]*list.Element), order: list.New()}
	go rl.sweepLoop(limiterSweepInterval)
	return rl
}

func (rl *authFlowLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if el, ok := rl.limiters[key]; ok {
		v := el.Value.(*visitor)
		v.lastSeen = time.Now()
		rl.order.MoveToBack(el)
		return v.limiter.Allow()
	}

	// 10 attempts per minute, burst 10 — this endpoint is already
	// requireAuth-gated and each hit just triggers an Authentik redirect,
	// so this is light hygiene throttling, not a brute-force defense.
	v := &visitor{key: key, limiter: rate.NewLimiter(rate.Every(6*time.Second), 10), lastSeen: time.Now()}
	rl.limiters[key] = rl.order.PushBack(v)
	for rl.order.Len() > maxEntries {
		rl.evictOldest()
	}
	return v.limiter.Allow()
}

// evictOldest drops the least-recently-used bucket. Caller holds rl.mu.
func (rl *authFlowLimiter) evictOldest() {
	el := rl.order.Front()
	if el == nil {
		return
	}
	rl.order.Remove(el)
	delete(rl.limiters, el.Value.(*visitor).key)
}

// sweep reclaims idle buckets. Runs off the request path.
func (rl *authFlowLimiter) sweep(now time.Time) int {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	removed := 0
	for el := rl.order.Front(); el != nil; {
		v := el.Value.(*visitor)
		if now.Sub(v.lastSeen) <= visitorTTL {
			// order is LRU-first, so everything after this is newer.
			break
		}
		next := el.Next()
		rl.order.Remove(el)
		delete(rl.limiters, v.key)
		removed++
		el = next
	}
	return removed
}

func (rl *authFlowLimiter) sweepLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		if n := rl.sweep(time.Now()); n > 0 {
			slog.Debug("authFlowLimiter swept idle buckets", "removed", n)
		}
	}
}

// limiterKey prefers the authenticated user id, which a caller cannot forge.
func limiterKey(r *http.Request) string {
	if uid, ok := auth.UserIDFromCtx(r.Context()); ok && uid != 0 {
		return "user:" + strconv.FormatInt(uid, 10)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return "ip:" + host
}

func (rl *authFlowLimiter) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !rl.allow(limiterKey(r)) {
			Error(w, http.StatusTooManyRequests, "rate_limited", "too many attempts, try again later")
			return
		}
		next.ServeHTTP(w, r)
	})
}
