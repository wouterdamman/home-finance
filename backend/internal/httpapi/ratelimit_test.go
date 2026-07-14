package httpapi

import "testing"

func TestPasswordRateLimiterBlocksAfterBurst(t *testing.T) {
	rl := newPasswordRateLimiter()
	const key = "1.2.3.4"

	for i := 0; i < 5; i++ {
		if !rl.allow(key) {
			t.Fatalf("attempt %d: want allowed (within burst), got blocked", i)
		}
	}
	if rl.allow(key) {
		t.Fatal("attempt 6: want blocked (burst exhausted), got allowed")
	}
}

func TestPasswordRateLimiterIsolatesByKey(t *testing.T) {
	rl := newPasswordRateLimiter()
	for i := 0; i < 5; i++ {
		rl.allow("1.1.1.1")
	}
	if !rl.allow("2.2.2.2") {
		t.Fatal("different key should have its own independent bucket")
	}
}
