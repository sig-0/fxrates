package server

import (
	"embed"
	"net/http"
)

//go:embed openapi.yaml
var openAPIFS embed.FS

func (s *Server) OpenAPI(w http.ResponseWriter, _ *http.Request) {
	b, err := openAPIFS.ReadFile("openapi.yaml")
	if err != nil {
		http.Error(w, "openapi not found", http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	_, _ = w.Write(b) //nolint:errcheck // Fine to ignore
}

func (s *Server) Redoc(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	//nolint:errcheck // Fine to ignore
	_, _ = w.Write([]byte(`<!doctype html>
<html>
  <head>
    <meta charset="utf-8"/>
    <meta name="viewport" content="width=device-width, initial-scale=1"/>
    <title>fxrates API</title>
  </head>
  <body>
    <redoc spec-url="/openapi.yaml"></redoc>
    <script src="https://cdn.redoc.ly/redoc/latest/bundles/redoc.standalone.js"></script>
  </body>
</html>`))
}
