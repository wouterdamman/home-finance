package config

import (
	"strings"
	"testing"
	"time"

	"github.com/wouterdamman/home-finance/internal/auth"
)

// The session TTLs are the one pair of settings where a wrong value fails
// silently rather than loudly: scs treats a non-positive duration as
// "already expired", so it would log everyone out on their next request,
// and an idle timeout above the lifetime simply never fires.
func TestLoadSessionTTLs(t *testing.T) {
	cases := []struct {
		name        string
		lifetime    string
		idleTimeout string
		wantErr     string
		wantLife    time.Duration
		wantIdle    time.Duration
	}{
		{
			name:     "defaults match the auth package",
			wantLife: auth.DefaultSessionLifetime,
			wantIdle: auth.DefaultSessionIdleTimeout,
		},
		{
			name:        "two weeks",
			lifetime:    "336h",
			idleTimeout: "336h",
			wantLife:    336 * time.Hour,
			wantIdle:    336 * time.Hour,
		},
		{
			name:     "zero lifetime is refused",
			lifetime: "0s",
			wantErr:  "SESSION_LIFETIME must be positive",
		},
		{
			name:        "negative idle timeout is refused",
			idleTimeout: "-1h",
			wantErr:     "SESSION_IDLE_TIMEOUT must be positive",
		},
		{
			name:        "idle timeout above lifetime is refused",
			lifetime:    "24h",
			idleTimeout: "48h",
			wantErr:     "must not exceed SESSION_LIFETIME",
		},
		{
			name:     "unparseable duration is refused",
			lifetime: "two weeks",
			// env.Parse rejects it before Load's own checks run, so the
			// message names the struct field rather than the env var.
			wantErr: `parse error on field "SessionLifetime"`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("DATABASE_URL", "postgres://localhost/test")
			if tc.lifetime != "" {
				t.Setenv("SESSION_LIFETIME", tc.lifetime)
			}
			if tc.idleTimeout != "" {
				t.Setenv("SESSION_IDLE_TIMEOUT", tc.idleTimeout)
			}

			cfg, err := Load()
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("Load() succeeded, want error containing %q", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("Load() error = %q, want it to contain %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load() = %v, want success", err)
			}
			if cfg.SessionLifetime != tc.wantLife {
				t.Errorf("SessionLifetime = %s, want %s", cfg.SessionLifetime, tc.wantLife)
			}
			if cfg.SessionIdleTimeout != tc.wantIdle {
				t.Errorf("SessionIdleTimeout = %s, want %s", cfg.SessionIdleTimeout, tc.wantIdle)
			}
		})
	}
}
