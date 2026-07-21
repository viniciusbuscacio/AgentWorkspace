package webserver

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"strings"
)

// webConfig is injected into index.html as window.aw_WEB so the React shim
// boots in web mode and talks to the bridge instead of the (absent) Wails
// runtime. api="" means "same origin"; token is empty until the user logs in.
type webConfig struct {
	API   string `json:"api"`
	Token string `json:"token"`
}

// configScript renders the inline <script> injected into index.html.
func configScript() string {
	encoded, err := json.Marshal(webConfig{API: "", Token: ""})
	if err != nil {
		return ""
	}
	return "<script>window.aw_WEB = " + string(encoded) + ";</script>"
}

// assetHandler serves the embedded React frontend. index.html is served with
// the window.aw_WEB config injected; everything else is served straight from
// dist. Returns nil when assets are not configured.
func assetHandler(dist fs.FS) http.Handler {
	if dist == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "web assets not available", http.StatusServiceUnavailable)
		})
	}
	fileServer := http.FileServer(http.FS(dist))
	script := configScript()
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
		injected := strings.Replace(string(html), "<head>", "<head>\n    "+script, 1)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte(injected))
	})
}
