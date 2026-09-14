package httpapi

import (
	"embed"
	"net/http"
)

//go:embed openapi.yaml
var openAPISpec embed.FS

// scalarScriptURL is pinned to an exact version and guarded by an SRI hash.
// This page is same-origin with the app and admin-only, so an unpinned CDN
// script would give any compromised publish of @scalar/api-reference full
// admin API access: it runs with the session cookie and can set the
// X-Requested-With header the CSRF check looks for.
//
// To bump: fetch the new URL and recompute the hash with
//
//	curl -s <url> | openssl dgst -sha384 -binary | openssl base64 -A
const (
	scalarScriptURL = "https://cdn.jsdelivr.net/npm/@scalar/api-reference@1.68.0/dist/browser/standalone.min.js"
	scalarScriptSRI = "sha384-ayGz8N+NChlUEfR0zr5Zy3T6Q4lhcdiASJNoshS6+vxV56ZE300qfWNBjj9pqsLN"
	scalarCDNOrigin = "https://cdn.jsdelivr.net"
	scalarFontsHost = "https://fonts.scalar.com"
)

// docsCSP is set here rather than in the global securityHeaders middleware,
// which deliberately skips /api/docs (the global policy is script-src 'self',
// which this page cannot use). connect-src stays 'self': the reference only
// ever fetches /api/openapi.yaml, so the bundle gets no route off-origin for
// anything it reads out of the admin API.
const docsCSP = "default-src 'none'; " +
	"script-src " + scalarCDNOrigin + "; " +
	"style-src 'self' 'unsafe-inline' " + scalarCDNOrigin + "; " +
	"font-src 'self' data: " + scalarCDNOrigin + " " + scalarFontsHost + "; " +
	"img-src 'self' data:; " +
	"connect-src 'self'; " +
	"object-src 'none'; " +
	"base-uri 'none'; " +
	"form-action 'none'; " +
	"frame-ancestors 'none'"

// apiDocsHTML embeds Scalar via CDN — no Go dependency, no bundling step.
const apiDocsHTML = `<!doctype html>
<html>
  <head>
    <title>Home Finance API</title>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
  </head>
  <body>
    <script id="api-reference" data-url="/api/openapi.yaml"></script>
    <script src="` + scalarScriptURL + `" integrity="` + scalarScriptSRI + `" crossorigin="anonymous"></script>
  </body>
</html>
`

func (s *Server) handleAPIDocs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", docsCSP)
	w.Write([]byte(apiDocsHTML))
}

func (s *Server) handleOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	data, err := openAPISpec.ReadFile("openapi.yaml")
	if err != nil {
		Error(w, http.StatusInternalServerError, "spec_missing", err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.Write(data)
}
