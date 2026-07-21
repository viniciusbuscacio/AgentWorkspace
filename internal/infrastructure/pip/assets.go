package pip

import (
	"io/fs"
	"net/http"
	"strings"
)

// AssetHandler serves the embedded React frontend for a PiP window. Every
// request is served from dist, except index.html, into which the
// window.aw_PIP config script is injected so the SPA boots in PiP mode and
// talks to the host app through the token-scoped localhost API. Keeping this
// HTTP wiring here (not in the composition root) keeps main.go free of raw I/O.
func AssetHandler(dist fs.FS, pipURL string) http.Handler {
	fileServer := http.FileServer(http.FS(dist))
	configScript := ConfigScript(pipURL)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && r.URL.Path != "/index.html" {
			fileServer.ServeHTTP(w, r)
			return
		}
		html, err := fs.ReadFile(dist, "index.html")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		injected := strings.Replace(string(html), "<head>", "<head>\n    "+configScript, 1)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(injected))
	})
}
