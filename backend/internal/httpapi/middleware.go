package httpapi

import (
	"net/http"
	"strings"
)

// maxJSONBody caps a JSON request body. json.Decoder buffers a whole top-level
// value before unmarshalling, so without this an uncapped body is fully
// resident in memory and a single request can OOM the pod. Generous for the
// largest legitimate payload (a category reorder carrying every category id).
const maxJSONBody = 1 << 20 // 1 MiB

// bodyLimit caps request bodies for the routes it wraps. Multipart requests are
// skipped: the xlsx-import and avatar-upload handlers install their own,
// deliberately larger, MaxBytesReader before parsing the form.
func bodyLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodDelete:
		default:
			if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
				r.Body = http.MaxBytesReader(w, r.Body, maxJSONBody)
			}
		}
		next.ServeHTTP(w, r)
	})
}

// contentSecurityPolicy for the SPA and API. The frontend bundle is entirely
// self-contained — no CDN, no external font, image or API host (verified across
// index.html and the whole src tree) — so 'self' needs no exceptions. Mantine
// injects runtime <style> elements, hence 'unsafe-inline' for styles only;
// scripts need no such relaxation because the Vite production build evaluates
// no inline or dynamic code.
const contentSecurityPolicy = "default-src 'self'; " +
	"script-src 'self'; " +
	"style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data: blob:; " +
	"font-src 'self' data:; " +
	"connect-src 'self'; " +
	"object-src 'none'; " +
	"base-uri 'none'; " +
	"form-action 'self'; " +
	"frame-ancestors 'none'"

// securityHeaders sets the response headers the app previously shipped without
// entirely. frame-ancestors/X-Frame-Options is the load-bearing one: most
// mutations are not step-up-reauth gated, so without it a logged-in user can be
// clickjacked into a one-click destructive action through a framing page — and
// the X-Requested-With CSRF control does not help there, because the click
// happens inside the real origin.
//
// /api/docs sets its own policy (it renders a third-party API-reference bundle)
// and is skipped here.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/docs" {
			w.Header().Set("Content-Security-Policy", contentSecurityPolicy)
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), interest-cohort=()")
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		next.ServeHTTP(w, r)
	})
}

// apiNotFound returns a JSON 404 for unmatched /api paths. chi propagates the
// root router's NotFound handler into already-registered sub-routers, so
// without this an unknown /api path falls through to the SPA handler and
// answers 200 + index.html — which a fetch client then fails to parse as JSON,
// reporting a syntax error instead of a missing endpoint.
func apiNotFound(w http.ResponseWriter, r *http.Request) {
	Error(w, http.StatusNotFound, "not_found", "no such endpoint")
}

// apiMethodNotAllowed keeps the sibling failure mode consistent: chi's default
// 405 writes an empty body, so a client sees neither JSON nor an explanation.
func apiMethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	Error(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed for this endpoint")
}
