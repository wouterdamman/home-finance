package httpapi

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

const (
	maxKidSlugLen = 64
	// reservedKidSlug is a static route under /kids and would otherwise be
	// shadowed by (or shadow) a kid with that name.
	reservedKidSlug = "balances"
)

var kidSlugPathRe = regexp.MustCompile(`^[a-z0-9-]{1,64}$`)

// validKidSlugPath reports whether a path segment can be a kid slug at all.
func validKidSlugPath(s string) bool { return kidSlugPathRe.MatchString(s) }

// letters that do not decompose under NFD.
var slugFold = map[rune]string{
	'ß': "ss", 'æ': "ae", 'œ': "oe", 'ø': "o", 'đ': "d", 'ð': "d",
	'ł': "l", 'þ': "th", 'ı': "i",
}

// slugify lowercases, folds diacritics, collapses every run of characters
// outside [a-z0-9] into a single '-', trims dashes and caps the length.
// It may return "".
func slugify(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range norm.NFD.String(strings.ToLower(name)) {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		if f, ok := slugFold[r]; ok {
			b.WriteString(f)
			dash = false
			continue
		}
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
		} else if !dash {
			b.WriteByte('-')
			dash = true
		}
	}
	return trimSlug(b.String(), maxKidSlugLen)
}

func trimSlug(s string, max int) string {
	s = strings.Trim(s, "-")
	if len(s) > max {
		s = strings.Trim(s[:max], "-")
	}
	return s
}

type slugInput struct {
	ID   int64
	Name string
}

// assignKidSlugs derives the stable public key for each (active) kid.
// Kids whose slug is empty get "kid-<id>", the reserved slug gets
// "balances-<id>", and every kid in a group of colliding slugs gets
// "<slug>-<id>". Result is keyed by kid id.
func assignKidSlugs(kids []slugInput) map[int64]string {
	return assignSlugs(kids, []string{reservedKidSlug}, "kid")
}

// assignSlugs is the shared slug assignment: items whose slug is empty or
// reserved get "<slug>-<id>" ("<emptyPrefix>-<id>" when empty), and every
// item in a group of colliding slugs gets "<slug>-<id>". Result is keyed by id.
func assignSlugs(items []slugInput, reserved []string, emptyPrefix string) map[int64]string {
	base := make(map[int64]string, len(items))
	count := map[string]int{}
	for _, k := range items {
		s := slugify(k.Name)
		if s == "" || slices.Contains(reserved, s) {
			s = suffixSlug(s, k.ID, emptyPrefix)
		} else {
			count[s]++
		}
		base[k.ID] = s
	}
	out := make(map[int64]string, len(items))
	final := map[string]int{}
	for _, k := range items {
		s := base[k.ID]
		if count[s] > 1 {
			s = suffixSlug(s, k.ID, emptyPrefix)
		}
		out[k.ID] = s
		final[s]++
	}
	// Pathological: a suffixed slug equals another item's slug (e.g. a name
	// literally "Anna 7"). Fall back to the always-unique <emptyPrefix>-<id>.
	for _, k := range items {
		if final[out[k.ID]] > 1 {
			out[k.ID] = emptyPrefix + "-" + strconv.FormatInt(k.ID, 10)
		}
	}
	return out
}

func suffixSlug(s string, id int64, empty string) string {
	suffix := "-" + strconv.FormatInt(id, 10)
	if s == "" {
		return empty + suffix
	}
	return trimSlug(s, maxKidSlugLen-len(suffix)) + suffix
}
