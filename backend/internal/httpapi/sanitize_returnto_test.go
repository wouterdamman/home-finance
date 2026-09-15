package httpapi

import "testing"

func TestSanitizeReturnTo(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{name: "plain path", in: "/periods/1", want: "/periods/1"},
		{name: "path with query survives", in: "/periods/1?tab=transactions&x=2", want: "/periods/1?tab=transactions&x=2"},
		{name: "empty", in: "", want: "/"},
		// Protocol-relative: no scheme, so url.IsAbs() is false, yet every
		// browser resolves these off-site.
		{name: "protocol relative", in: "//evil.com", want: "/"},
		{name: "triple slash", in: "///evil.com", want: "/"},
		// Chrome and Edge normalize a backslash to a forward slash here.
		{name: "backslash protocol relative", in: `/\evil.com`, want: "/"},
		{name: "mixed slash protocol relative", in: `/\/evil.com`, want: "/"},
		{name: "absolute url", in: "https://evil.com", want: "/"},
		{name: "javascript scheme", in: "javascript:alert(1)", want: "/"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sanitizeReturnTo(tc.in); got != tc.want {
				t.Errorf("sanitizeReturnTo(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
