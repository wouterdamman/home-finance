package httpapi

import (
	"strings"
	"testing"
)

func TestSlugify(t *testing.T) {
	for in, want := range map[string]string{
		"Lize":              "lize",
		"Zoë":               "zoe",
		"  Anne-Marie  ":    "anne-marie",
		"Jan  &  Piet!":     "jan-piet",
		"Ærøskøbing Straße": "aeroskobing-strasse",
		"Łukasz":            "lukasz",
		"Noël 2":            "noel-2",
		"!!!":               "",
		"":                  "",
		"日本":                "",
	} {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
	long := slugify(strings.Repeat("ab ", 40))
	if len(long) > 64 || strings.HasSuffix(long, "-") || !validKidSlugPath(long) {
		t.Errorf("long slug %q (%d)", long, len(long))
	}
}

func TestAssignKidSlugs(t *testing.T) {
	got := assignKidSlugs([]slugInput{
		{1, "Lize"}, {2, "Zoë"}, {3, "Sam"}, {4, "sam"}, {5, "Balances"},
		{6, "???"}, {7, "Anna"}, {8, "Anna 7"}, {9, "Solo"},
	})
	want := map[int64]string{
		1: "lize", 2: "zoe", 3: "sam-3", 4: "sam-4", 5: "balances-5",
		6: "kid-6", 7: "anna", 8: "anna-7", 9: "solo",
	}
	// 7 ("anna") and 8 ("anna-7") do not collide as base slugs; fine.
	for id, w := range want {
		if got[id] != w {
			t.Errorf("kid %d: got %q, want %q", id, got[id], w)
		}
	}
	seen := map[string]bool{}
	for _, s := range got {
		if seen[s] || !validKidSlugPath(s) {
			t.Errorf("duplicate or invalid slug %q", s)
		}
		seen[s] = true
	}

	// Suffixed slug equals another kid's slug -> fall back to kid-<id>.
	got = assignKidSlugs([]slugInput{{7, "Anna"}, {8, "Anna"}, {9, "Anna 7"}})
	if got[9] == got[7] || got[7] == got[8] {
		t.Errorf("collision not resolved: %v", got)
	}

	// Long colliding names stay within 64 chars.
	n := strings.Repeat("x", 100)
	got = assignKidSlugs([]slugInput{{123, n}, {124, n}})
	for _, s := range got {
		if len(s) > 64 || !validKidSlugPath(s) {
			t.Errorf("bad long slug %q", s)
		}
	}
}

func TestValidKidSlugPath(t *testing.T) {
	for s, ok := range map[string]bool{
		"lize": true, "a-b-1": true, "": false, "Lize": false, "a b": false,
		"zoë": false, "a/b": false, strings.Repeat("a", 64): true, strings.Repeat("a", 65): false,
	} {
		if validKidSlugPath(s) != ok {
			t.Errorf("validKidSlugPath(%q) != %v", s, ok)
		}
	}
}

func TestAssignSlugsPots(t *testing.T) {
	got := assignSlugs([]slugInput{
		{1, "Vakantie"}, {2, "Balances"}, {3, "???"}, {4, "Auto"}, {5, "auto"}, {6, "Vakantie 1"},
	}, nil, "pot")
	want := map[int64]string{1: "vakantie", 2: "balances", 3: "pot-3", 4: "auto-4", 5: "auto-5", 6: "vakantie-1"}
	for id, w := range want {
		if got[id] != w {
			t.Errorf("pot %d: got %q, want %q", id, got[id], w)
		}
	}
	// Suffixed slug equals another pot's slug -> pot-<id> fallback.
	got = assignSlugs([]slugInput{{7, "Anna"}, {8, "Anna"}, {9, "Anna 7"}}, nil, "pot")
	if got[9] == got[7] || got[7] == got[8] {
		t.Errorf("collision not resolved: %v", got)
	}
	// Reserved words only apply when passed in.
	if got := assignSlugs([]slugInput{{1, "Balances"}}, []string{"balances"}, "pot"); got[1] != "balances-1" {
		t.Errorf("reserved: %v", got)
	}
}
