package httpapi

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// spaFixture lays out a static dir with a secret sitting next to it, so a
// successful traversal is unambiguous in the response body.
func spaFixture(t *testing.T) (staticDir string) {
	t.Helper()
	base := t.TempDir()
	staticDir = filepath.Join(base, "static")
	if err := os.Mkdir(staticDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "secret"), []byte("TOP-SECRET"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staticDir, "index.html"), []byte("<!doctype html>SPA"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(staticDir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staticDir, "assets", "app.js"), []byte("console.log(1)"), 0o644); err != nil {
		t.Fatal(err)
	}
	return staticDir
}

func TestSPAHandlerServesRealFiles(t *testing.T) {
	h := SPAHandler(spaFixture(t))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/assets/app.js", nil))
	if body := rec.Body.String(); body != "console.log(1)" {
		t.Errorf("got %q, want the asset contents", body)
	}
}

func TestSPAHandlerFallsBackToIndex(t *testing.T) {
	h := SPAHandler(spaFixture(t))
	for _, p := range []string{"/", "/pots/3", "/assets/"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		if !strings.Contains(rec.Body.String(), "SPA") {
			t.Errorf("%s: got %q, want index.html", p, rec.Body.String())
		}
	}
}

func TestSPAHandlerRejectsTraversal(t *testing.T) {
	h := SPAHandler(spaFixture(t))
	targets := []string{
		"/../secret",
		"/..%2fsecret",
		"/assets/../../secret",
		"/a/b/c/../../../../secret",
		"/./../secret",
		"/%2e%2e/secret",
	}
	for _, target := range targets {
		t.Run(target, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
			if strings.Contains(rec.Body.String(), "TOP-SECRET") {
				t.Fatalf("traversal succeeded: %q", rec.Body.String())
			}
		})
	}
}

func TestResolveStaticPathContainment(t *testing.T) {
	root := "/srv/static"
	cases := []struct {
		urlPath string
		want    string
		ok      bool
	}{
		{"/assets/app.js", "/srv/static/assets/app.js", true},
		{"/../secret", "/srv/static/secret", true},            // clamped, not escaped
		{"/../../etc/passwd", "/srv/static/etc/passwd", true}, // clamped, not escaped
		{"/", "", false},
		{"/assets/", "", false},
	}
	for _, tc := range cases {
		got, ok := resolveStaticPath(root, tc.urlPath)
		if ok != tc.ok || got != tc.want {
			t.Errorf("resolveStaticPath(%q) = (%q, %v), want (%q, %v)", tc.urlPath, got, ok, tc.want, tc.ok)
		}
	}
}
