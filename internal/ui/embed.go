// Package ui exposes the embedded frontend assets built by Vite into web/dist.
// In production builds (default), assets come from the embedded filesystem.
// In dev builds (//go:build dev), requests are reverse-proxied to the Vite dev
// server on :5173 — see fs_dev.go.
package ui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distFS embed.FS

// DistFS returns the embedded web/dist filesystem rooted at "dist".
// The returned fs.FS serves files relative to dist/ (i.e., "index.html" not
// "dist/index.html"). Used by the production handler in fs_prod.go.
func DistFS() fs.FS {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		// Embed guarantees this path exists at compile time; unreachable.
		panic("ui: embed dist subtree missing: " + err.Error())
	}
	return sub
}
