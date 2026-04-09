//go:build !dev

package ui

import (
	"net/http"
)

// Handler returns the production asset handler: serves files from the embedded
// web/dist filesystem. The dev override in fs_dev.go replaces this at build
// time via the `dev` build tag.
func Handler() http.Handler {
	return http.FileServerFS(DistFS())
}
