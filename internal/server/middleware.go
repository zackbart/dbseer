package server

import (
	"crypto/subtle"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
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

func basicAuthGuard(cfg AuthConfig) func(http.Handler) http.Handler {
	realm := `Basic realm="dbseer"`
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, pass, ok := r.BasicAuth()
			if !ok ||
				subtle.ConstantTimeCompare([]byte(user), []byte(cfg.Username)) != 1 ||
				subtle.ConstantTimeCompare([]byte(pass), []byte(cfg.Password)) != 1 {
				w.Header().Set("WWW-Authenticate", realm)
				writeError(w, http.StatusUnauthorized, ErrCodeInvalidRequest, "HTTP auth required", nil)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func csrfCookie(cfg AuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.SetCookie(w, &http.Cookie{
				Name:     csrfCookieName,
				Value:    cfg.CSRFToken,
				Path:     "/",
				HttpOnly: false,
				SameSite: http.SameSiteStrictMode,
				Secure:   r.TLS != nil,
			})
			next.ServeHTTP(w, r)
		})
	}
}

func csrfGuard(cfg AuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !isMutationMethod(r.Method) {
				next.ServeHTTP(w, r)
				return
			}

			cookie, err := r.Cookie(csrfCookieName)
			if err != nil || cookie.Value == "" {
				writeError(w, http.StatusForbidden, ErrCodeInvalidRequest, "missing CSRF cookie", nil)
				return
			}

			header := r.Header.Get("X-Dbseer-CSRF")
			if subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(cfg.CSRFToken)) != 1 ||
				subtle.ConstantTimeCompare([]byte(header), []byte(cfg.CSRFToken)) != 1 {
				writeError(w, http.StatusForbidden, ErrCodeInvalidRequest, "invalid CSRF token", nil)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// sameOriginMutationGuard rejects cross-site mutation requests to the row
// endpoints. Browser clients are expected to send Origin or Referer.
func sameOriginMutationGuard() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasPrefix(r.URL.Path, "/api/tables/") || !isMutationMethod(r.Method) {
				next.ServeHTTP(w, r)
				return
			}

			if secFetchSite := strings.ToLower(r.Header.Get("Sec-Fetch-Site")); secFetchSite == "cross-site" {
				writeError(w, http.StatusForbidden, ErrCodeInvalidRequest, "cross-site mutation blocked", nil)
				return
			}

			if err := validateSameOriginRequest(r); err != nil {
				writeError(w, http.StatusForbidden, ErrCodeInvalidRequest, err.Error(), nil)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func isMutationMethod(method string) bool {
	return method == http.MethodPost || method == http.MethodPatch || method == http.MethodDelete || method == http.MethodPut
}

func validateSameOriginRequest(r *http.Request) error {
	if origin := r.Header.Get("Origin"); origin != "" {
		if !sameOrigin(origin, r) {
			return fmt.Errorf("origin must match the dbseer host")
		}
		return nil
	}
	if referer := r.Header.Get("Referer"); referer != "" {
		if !sameOrigin(referer, r) {
			return fmt.Errorf("referer must match the dbseer host")
		}
	}
	return nil
}

func sameOrigin(raw string, r *http.Request) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return false
	}
	return strings.EqualFold(u.Host, r.Host)
}

// securityHeaders returns middleware that sets security-related HTTP headers.
func securityHeaders() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
			w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
			w.Header().Set("Referrer-Policy", "same-origin")
			w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
			next.ServeHTTP(w, r)
		})
	}
}

// maxBodySize returns middleware that enforces a maximum request body size.
func maxBodySize(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > maxBytes {
				writeError(w, http.StatusRequestEntityTooLarge, ErrCodePayloadTooLarge, "request body too large", nil)
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}
