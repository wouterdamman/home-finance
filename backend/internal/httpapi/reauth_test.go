package httpapi

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"

	"github.com/wouterdamman/home-finance/internal/config"
)

func newTestSessionCtx(t *testing.T) (*scs.SessionManager, context.Context) {
	t.Helper()
	sm := scs.New()
	ctx, err := sm.Load(context.Background(), "")
	if err != nil {
		t.Fatalf("load session: %v", err)
	}
	return sm, ctx
}

func TestReauthIsFreshTiming(t *testing.T) {
	cases := []struct {
		name string
		age  time.Duration // 0 with unset=true means "never set"
		set  bool
		want bool
	}{
		{name: "just now", age: 0, set: true, want: true},
		{name: "4 minutes ago", age: 4 * time.Minute, set: true, want: true},
		{name: "6 minutes ago", age: 6 * time.Minute, set: true, want: false},
		{name: "never set", set: false, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sm, ctx := newTestSessionCtx(t)
			if tc.set {
				sm.Put(ctx, "reauthAt", time.Now().Add(-tc.age))
			}
			if got := reauthIsFresh(sm, ctx); got != tc.want {
				t.Errorf("reauthIsFresh() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRequireFreshReauth(t *testing.T) {
	t.Run("dev fake auth always bypasses", func(t *testing.T) {
		sm, ctx := newTestSessionCtx(t)
		s := &Server{cfg: &config.Config{DevFakeAuth: true}, sm: sm}
		r := httptest.NewRequest("POST", "/", nil).WithContext(ctx)
		w := httptest.NewRecorder()
		if !s.requireFreshReauth(w, r) {
			t.Error("expected requireFreshReauth to bypass under DevFakeAuth")
		}
		if w.Code != 200 {
			t.Errorf("expected no response written, got status %d", w.Code)
		}
	})

	t.Run("fresh reauth passes", func(t *testing.T) {
		sm, ctx := newTestSessionCtx(t)
		sm.Put(ctx, "reauthAt", time.Now())
		s := &Server{cfg: &config.Config{DevFakeAuth: false}, sm: sm}
		r := httptest.NewRequest("POST", "/", nil).WithContext(ctx)
		w := httptest.NewRecorder()
		if !s.requireFreshReauth(w, r) {
			t.Error("expected requireFreshReauth to pass with a fresh reauthAt")
		}
	})

	t.Run("stale reauth is rejected with reauth_required", func(t *testing.T) {
		sm, ctx := newTestSessionCtx(t)
		sm.Put(ctx, "reauthAt", time.Now().Add(-10*time.Minute))
		s := &Server{cfg: &config.Config{DevFakeAuth: false}, sm: sm}
		r := httptest.NewRequest("POST", "/", nil).WithContext(ctx)
		w := httptest.NewRecorder()
		if s.requireFreshReauth(w, r) {
			t.Fatal("expected requireFreshReauth to reject a stale reauthAt")
		}
		if w.Code != 401 {
			t.Errorf("expected 401, got %d", w.Code)
		}
		if got := w.Body.String(); !strings.Contains(got, "reauth_required") {
			t.Errorf("expected body to contain reauth_required, got %q", got)
		}
	})

	t.Run("missing reauth is rejected", func(t *testing.T) {
		sm, ctx := newTestSessionCtx(t)
		s := &Server{cfg: &config.Config{DevFakeAuth: false}, sm: sm}
		r := httptest.NewRequest("POST", "/", nil).WithContext(ctx)
		w := httptest.NewRecorder()
		if s.requireFreshReauth(w, r) {
			t.Fatal("expected requireFreshReauth to reject when reauthAt was never set")
		}
	})
}

func TestVerifyReauthIdentity(t *testing.T) {
	cases := []struct {
		name                   string
		storedSub, storedEmail string
		claimsSub, claimsEmail string
		wantErr                bool
	}{
		{
			name: "sub matches", storedSub: "sub-1", storedEmail: "a@example.com",
			claimsSub: "sub-1", claimsEmail: "different@example.com", wantErr: false,
		},
		{
			name: "sub mismatch, email also mismatches", storedSub: "sub-1", storedEmail: "a@example.com",
			claimsSub: "sub-2", claimsEmail: "b@example.com", wantErr: true,
		},
		{
			name: "sub mismatch but email matches case-insensitively", storedSub: "sub-1", storedEmail: "A@Example.com",
			claimsSub: "sub-2", claimsEmail: "a@example.com", wantErr: false,
		},
		{
			name: "both mismatch", storedSub: "sub-1", storedEmail: "a@example.com",
			claimsSub: "sub-2", claimsEmail: "c@example.com", wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := verifyReauthIdentity(tc.storedSub, tc.storedEmail, tc.claimsSub, tc.claimsEmail)
			if (err != nil) != tc.wantErr {
				t.Errorf("verifyReauthIdentity() err = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
