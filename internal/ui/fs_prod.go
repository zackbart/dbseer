//go:build !dev

package ui

import (
	"io/fs"
	"net/http"
	"strings"
)

// Handler returns the production asset handler: serves files from the
// embedded web/dist filesystem with an SPA fallback.
//
// Behavior:
//   - Existing files (e.g. /assets/index-xxx.js, /index.html) are served
//     directly by http.FileServerFS.
//   - Requests for paths that do not exist in the embedded FS (e.g. client-
//     side routes like /t/app.users) fall back to serving index.html so the
//     React Router can take over on the client.
//   - Requests under /api/ are NEVER fallen back — they get the file
//     server's natural 404 — so a misconfigured router can't accidentally
//     mask API errors with HTML responses. (chi already routes /api before
//     this handler; this is defense in depth.)
//
// The dev override in fs_dev.go replaces this at build time via the `dev`
// build tag and reverse-proxies to the Vite dev server instead.
func Handler() http.Handler {
	dist := DistFS()
	fileServer := http.FileServerFS(dist)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")

		// Defense in depth: never serve index.html for /api/ paths.
		if strings.HasPrefix(p, "api/") {
			fileServer.ServeHTTP(w, r)
			return
		}

		// Empty path or explicit index — let the file server handle it.
		if p == "" || p == "index.html" {
			fileServer.ServeHTTP(w, r)
			return
		}

		// If the path resolves to a real embedded asset, serve it directly.
		if _, err := fs.Stat(dist, p); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}

		// Unknown path → SPA fallback. Rewrite the request to "/" so the
		// file server returns index.html; the client-side router then
		// matches the original URL from window.location.
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/"
		fileServer.ServeHTTP(w, r2)
	})
}
