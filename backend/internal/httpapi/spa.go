package httpapi

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// SPAHandler serves the built frontend, falling back to index.html so client
// routes resolve.
//
// filepath.Join(staticDir, r.URL.Path) is the textbook traversal bug: it was
// safe here only because http.ServeFile rejects a path containing "..", a
// guard this code neither referenced nor tested. resolveStaticPath makes the
// containment explicit.
func SPAHandler(staticDir string) http.Handler {
	root := staticDir
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	// Resolved once, so the prefix check below compares like with like when
	// STATIC_DIR itself is a symlink (a k8s configmap/volume mount often is).
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	index := filepath.Join(root, "index.html")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p, ok := resolveStaticPath(root, r.URL.Path); ok {
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				http.ServeFile(w, r, p)
				return
			}
		}
		http.ServeFile(w, r, index)
	})
}

// resolveStaticPath maps a request path onto a file under root, reporting false
// if it would escape. r.URL.Path is already percent-decoded, so "..%2f" reaches
// here as a real "..".
func resolveStaticPath(root, urlPath string) (string, bool) {
	if urlPath == "" || strings.HasSuffix(urlPath, "/") {
		return "", false
	}
	p := filepath.Join(root, filepath.FromSlash(path.Clean("/"+urlPath)))
	if p != root && !strings.HasPrefix(p, root+string(filepath.Separator)) {
		return "", false
	}
	return p, true
}
