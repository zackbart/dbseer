package server

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"
)

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w, status: http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.wrote {
		rw.status = code
		rw.wrote = true
	}
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.wrote {
		rw.wrote = true
	}
	return rw.ResponseWriter.Write(b)
}

// requestLogger returns middleware that logs method, path, status, and duration
// at slog.Info level.
func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := newResponseWriter(w)
			next.ServeHTTP(rw, r)
			logger.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rw.status,
				"duration", time.Since(start).String(),
			)
		})
	}
}

// recoverer returns middleware that recovers from panics, logs at slog.Error,
// and returns a 500 internal error response.
func recoverer(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					stack := debug.Stack()
					logger.Error("panic recovered",
						"panic", rec,
						"stack", string(stack),
					)
					writeError(w, http.StatusInternalServerError, "internal", "internal server error", nil)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// readonlyGuard returns middleware that rejects non-GET/HEAD requests under
// /api/tables/ with 403 server_readonly when readonly is true.
func readonlyGuard(readonly bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if readonly &&
				r.Method != http.MethodGet &&
				r.Method != http.MethodHead &&
				strings.HasPrefix(r.URL.Path, "/api/tables/") {
				writeError(w, http.StatusForbidden, "server_readonly", "server is in read-only mode", nil)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
