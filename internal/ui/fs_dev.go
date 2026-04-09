//go:build dev

package ui

import (
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
)

// Handler (dev variant) reverse-proxies requests to the Vite dev server on
// :5173 so HMR works while iterating on the frontend. This file is only
// compiled when `-tags dev` is passed to `go build` — `air` does this via
// .air.toml, and `make dev` wraps air + `pnpm dev`.
func Handler() http.Handler {
	target, _ := url.Parse("http://localhost:5173")
	slog.Info("ui: dev mode — reverse-proxying to vite", "target", target.String())
	return httputil.NewSingleHostReverseProxy(target)
}
