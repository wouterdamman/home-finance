package auth

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

func TestNewIntegrationToken(t *testing.T) {
	raw, hash, prefix, err := NewIntegrationToken()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(raw, "hf_") {
		t.Errorf("token %q lacks hf_ prefix", raw)
	}
	b, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(raw, "hf_"))
	if err != nil || len(b) != 32 {
		t.Errorf("token body: want 32 base64url bytes, got %d (err %v)", len(b), err)
	}
	if prefix != raw[:8] || len(prefix) != 8 {
		t.Errorf("prefix %q is not the first 8 chars of the token", prefix)
	}
	if !bytes.Equal(hash, HashIntegrationToken(raw)) || len(hash) != 32 {
		t.Error("returned hash does not match HashIntegrationToken(raw)")
	}
	raw2, _, _, _ := NewIntegrationToken()
	if raw == raw2 {
		t.Error("two tokens were identical")
	}
}

func TestValidIntegrationScope(t *testing.T) {
	for s, want := range map[string]bool{"kids:read": true, "": false, "KIDS:READ": false, "pots:read": true, "years:read": true, "periods:read": true, "kids:write": false} {
		if got := ValidIntegrationScope(s); got != want {
			t.Errorf("ValidIntegrationScope(%q) = %v, want %v", s, got, want)
		}
	}
}

func TestBearerToken(t *testing.T) {
	cases := []struct {
		header string
		want   string
		ok     bool
	}{
		{"Bearer abc", "abc", true},
		{"bearer abc", "abc", true},
		{"BEARER abc", "abc", true},
		{"Bearer   abc", "abc", true},
		{"", "", false},
		{"Bearer", "", false},
		{"Bearer ", "", false},
		{"Basic abc", "", false},
		{"abc", "", false},
		{"Bearer abc def", "", false},
	}
	for _, c := range cases {
		got, ok := bearerToken(c.header)
		if got != c.want || ok != c.ok {
			t.Errorf("bearerToken(%q) = (%q,%v), want (%q,%v)", c.header, got, ok, c.want, c.ok)
		}
	}
}
