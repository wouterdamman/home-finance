package httpapi

import (
	"embed"
	"net/http"
)

//go:embed openapi.yaml
var openAPISpec embed.FS

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
    <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
  </body>
</html>
`

func (s *Server) handleAPIDocs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
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
