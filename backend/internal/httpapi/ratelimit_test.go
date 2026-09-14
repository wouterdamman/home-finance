package httpapi

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/wouterdamman/home-finance/internal/auth"
)

func TestAuthFlowLimiterBlocksAfterBurst(t *testing.T) {
	rl := newAuthFlowLimiter()
	const key = "1.2.3.4"

	for i := 0; i < 10; i++ {
		if !rl.allow(key) {
			t.Fatalf("attempt %d: want allowed (within burst), got blocked", i)
		}
	}
	if rl.allow(key) {
		t.Fatal("attempt 11: want blocked (burst exhausted), got allowed")
	}
}

func TestAuthFlowLimiterIsolatesByKey(t *testing.T) {
	rl := newAuthFlowLimiter()
	for i := 0; i < 10; i++ {
		rl.allow("1.1.1.1")
	}
	if !rl.allow("2.2.2.2") {
		t.Fatal("different key should have its own independent bucket")
	}
}

// middleware.RealIP rewrites RemoteAddr from caller-supplied headers, so an
// IP-keyed bucket is trivially rotated. An authenticated request must land in
// the same bucket no matter what address it appears to come from.
func TestAuthFlowLimiterKeyedOnUserNotSpoofableAddr(t *testing.T) {
	rl := newAuthFlowLimiter()
	// devFakeAuth=true puts a fixed user id in the request context, which is
	// what a real session does on this route (it sits behind requireAuth).
	h := auth.Require(nil, true)(rl.middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	for i := 0; i < 10; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/auth/reauth", nil)
		req.RemoteAddr = fmt.Sprintf("10.0.%d.%d:1234", i, i)
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("attempt %d: got %d, want 200", i, rec.Code)
		}
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/reauth", nil)
	req.RemoteAddr = "203.0.113.9:1234"
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("rotating the source address evaded the limit: got %d, want 429", rec.Code)
	}
}

func TestLimiterKeyFallsBackToRemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/auth/reauth", nil)
	req.RemoteAddr = "1.2.3.4:5678"
	if got := limiterKey(req); got != "ip:1.2.3.4" {
		t.Errorf("limiterKey = %q, want %q", got, "ip:1.2.3.4")
	}

	bare := httptest.NewRequest(http.MethodGet, "/auth/reauth", nil)
	bare.RemoteAddr = "unix"
	if got := limiterKey(bare); got != "ip:unix" {
		t.Errorf("limiterKey = %q, want %q", got, "ip:unix")
	}
}

func TestAuthFlowLimiterEvictsOldestPastCap(t *testing.T) {
	rl := newAuthFlowLimiter()
	for i := 0; i < maxEntries+50; i++ {
		rl.allow(fmt.Sprintf("ip:10.0.0.%d", i))
	}
	rl.mu.Lock()
	size, listLen := len(rl.limiters), rl.order.Len()
	_, oldestStillThere := rl.limiters["ip:10.0.0.0"]
	_, newestThere := rl.limiters[fmt.Sprintf("ip:10.0.0.%d", maxEntries+49)]
	rl.mu.Unlock()

	if size > maxEntries {
		t.Errorf("map grew to %d, want at most %d", size, maxEntries)
	}
	if listLen != size {
		t.Errorf("order list (%d) out of sync with map (%d)", listLen, size)
	}
	if oldestStillThere {
		t.Error("least-recently-used bucket should have been evicted")
	}
	if !newestThere {
		t.Error("most recent bucket should be present")
	}
}

func TestAuthFlowLimiterSweepDropsIdleBuckets(t *testing.T) {
	rl := newAuthFlowLimiter()
	rl.allow("ip:1.1.1.1")
	rl.allow("ip:2.2.2.2")

	if n := rl.sweep(time.Now()); n != 0 {
		t.Errorf("fresh buckets swept: %d", n)
	}
	if n := rl.sweep(time.Now().Add(visitorTTL + time.Minute)); n != 2 {
		t.Errorf("swept %d idle buckets, want 2", n)
	}
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if len(rl.limiters) != 0 || rl.order.Len() != 0 {
		t.Errorf("after sweep: map=%d list=%d, want 0/0", len(rl.limiters), rl.order.Len())
	}
}
