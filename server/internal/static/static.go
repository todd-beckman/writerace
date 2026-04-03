package static

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:dist
var dist embed.FS

// Content is the embedded frontend filesystem rooted at dist/.
// It is nil-safe for use in handler registration: when dist/ is empty
// (e.g. during development without a frontend build), callers should
// check whether serving static files is appropriate.
var Content, _ = fs.Sub(dist, "dist")

// Handler returns an http.Handler that serves the embedded frontend files
// with SPA fallback: requests for paths that don't match a real file
// receive index.html so client-side routing works.
func Handler(content fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(content))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to open the requested file. If it exists, serve it directly.
		path := r.URL.Path
		if path == "/" {
			fileServer.ServeHTTP(w, r)
			return
		}

		// Strip leading slash for fs.Open.
		file, err := content.Open(path[1:])
		if err == nil {
			file.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// File not found — serve index.html for SPA routing.
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
