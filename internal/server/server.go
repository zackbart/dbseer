// Package server implements the HTTP API layer for dbseer using chi.
// It composes the db, wire, safety, discover, and ui packages into a
// single http.Handler that can be mounted directly on a net/http server.
package server

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/zackbart/dbseer/internal/db"
	"github.com/zackbart/dbseer/internal/discover"
	"github.com/zackbart/dbseer/internal/safety"
	"github.com/zackbart/dbseer/internal/ui"
)

const csrfCookieName = "dbseer_csrf"

type AuthConfig struct {
	Username  string
	Password  string
	CSRFToken string
}

// Config holds the dependencies and settings for the HTTP server.
type Config struct {
	// Pool is the Postgres connection pool.
	Pool *db.Pool
	// Cache is the schema introspection cache.
	Cache *db.SchemaCache
	// Source is the discovered connection metadata returned by GET /api/discover.
	Source discover.Source
	// AuditLog is the audit logger. May be nil if audit logging is disabled.
	AuditLog *safety.Logger
	// Readonly mirrors Pool.Readonly(); cached here so middleware doesn't need the pool.
	Readonly bool
	// Auth, when non-nil, enables HTTP basic auth and CSRF protection.
	Auth *AuthConfig
	// Version is injected for /api response headers.
	Version string
	// Logger is the structured logger used for request and error logging.
	Logger *slog.Logger
}

// Server holds the constructed chi router and configuration.
type Server struct {
	cfg    Config
	router http.Handler
}

// New constructs a Server with a fully wired chi router.
func New(cfg Config) *Server {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	s := &Server{cfg: cfg}

	r := chi.NewRouter()

	// Global middleware.
	r.Use(requestLogger(cfg.Logger))
	r.Use(recoverer(cfg.Logger))
	r.Use(securityHeaders())
	r.Use(maxBodySize(2 << 20)) // 2MB max request body
	if cfg.Auth != nil {
		r.Use(basicAuthGuard(*cfg.Auth))
		r.Use(csrfCookie(*cfg.Auth))
		r.Use(csrfGuard(*cfg.Auth))
	}
	r.Use(sameOriginMutationGuard())
	r.Use(readonlyGuard(cfg.Readonly))

	// API routes.
	r.Route("/api", func(r chi.Router) {
		r.Get("/discover", s.handleDiscover)
		r.Get("/schema", s.handleSchema)

		r.Route("/tables/{schema}/{table}", func(r chi.Router) {
			r.Get("/rows", s.handleBrowse)
			r.Post("/rows", s.handleInsert)
			r.Patch("/rows", s.handleUpdate)
			r.Delete("/rows", s.handleDelete)
			r.Get("/fk-target", s.handleFKTarget)
		})

		r.Get("/history", s.handleHistory)
	})

	// Fallback: serve the embedded SPA for all non-/api paths.
	r.Handle("/*", middleware.SetHeader("Cache-Control", "no-cache")(ui.Handler()))

	s.router = r
	return s
}

// Handler returns the assembled http.Handler.
func (s *Server) Handler() http.Handler {
	return s.router
}
