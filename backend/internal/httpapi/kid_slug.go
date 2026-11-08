package httpapi

import (
	"regexp"
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
	base := make(map[int64]string, len(kids))
	count := map[string]int{}
	for _, k := range kids {
		s := slugify(k.Name)
		if s == "" || s == reservedKidSlug {
			s = suffixSlug(s, k.ID, "kid")
		} else {
			count[s]++
		}
		base[k.ID] = s
	}
	out := make(map[int64]string, len(kids))
	final := map[string]int{}
	for _, k := range kids {
		s := base[k.ID]
		if count[s] > 1 {
			s = suffixSlug(s, k.ID, "kid")
		}
		out[k.ID] = s
		final[s]++
	}
	// Pathological: a suffixed slug equals another kid's slug (e.g. a kid
	// literally named "Anna 7"). Fall back to the always-unique kid-<id>.
	for _, k := range kids {
		if final[out[k.ID]] > 1 {
			out[k.ID] = "kid-" + strconv.FormatInt(k.ID, 10)
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
