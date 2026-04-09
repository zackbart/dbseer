// Package safety implements the safety rails for dbseer: host classification,
// production-host detection, actionable error messages, connection validation,
// and the audit log appender.
package safety

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

// URLInfo holds the parsed components of a Postgres DSN.
type URLInfo struct {
	// Raw is the original DSN string as supplied by the caller.
	Raw string
	// Host is the canonical hostname (without port), empty string for unix sockets.
	Host string
	// Port is the port number as a string (e.g. "5432").
	Port string
	// Database is the target database name.
	Database string
	// User is the Postgres username.
	User string
	// IsLocalhost is true when the host resolves to a local loopback address or
	// unix socket. Applies to: "" (unix socket), "localhost", "127.0.0.1",
	// "::1", "0.0.0.0", and any host ending in ".local".
	IsLocalhost bool
}

// Parse accepts a Postgres DSN in either URL form ("postgres://...") or
// keyword form ("host=... port=... dbname=...") and returns a URLInfo.
// It delegates parsing to pgconn.ParseConfig to robustly handle both forms.
func Parse(dsn string) (URLInfo, error) {
	cfg, err := pgconn.ParseConfig(dsn)
	if err != nil {
		return URLInfo{}, fmt.Errorf("safety: parse DSN: %w", err)
	}

	host := cfg.Host
	port := fmt.Sprintf("%d", cfg.Port)

	info := URLInfo{
		Raw:         dsn,
		Host:        host,
		Port:        port,
		Database:    cfg.Database,
		User:        cfg.User,
		IsLocalhost: isLocalhost(host),
	}
	return info, nil
}

// isLocalhost returns true for hosts that are local loopback addresses or
// unix sockets.
func isLocalhost(host string) bool {
	switch host {
	case "", "localhost", "127.0.0.1", "::1", "0.0.0.0":
		return true
	}
	if strings.HasSuffix(host, ".local") {
		return true
	}
	return false
}

// Redact returns a safe string representation of the DSN suitable for logging.
// Any password in the DSN is replaced with "*****".
func (u URLInfo) Redact() string {
	// Try URL form first.
	if strings.HasPrefix(u.Raw, "postgres://") || strings.HasPrefix(u.Raw, "postgresql://") {
		parsed, err := url.Parse(u.Raw)
		if err == nil && parsed.User != nil {
			if _, hasPass := parsed.User.Password(); hasPass {
				parsed.User = url.UserPassword(parsed.User.Username(), "*****")
			}
			return parsed.String()
		}
	}
	// Keyword form: redact password=... value.
	return redactKeywordDSN(u.Raw)
}

// redactKeywordDSN replaces the value of the password key in a keyword-form DSN.
func redactKeywordDSN(dsn string) string {
	// Matches: password=<value> where value may be quoted or unquoted.
	// Use a simple state machine to avoid regex complexity with quoting.
	parts := strings.Fields(dsn)
	for i, part := range parts {
		if strings.HasPrefix(strings.ToLower(part), "password=") {
			parts[i] = "password=*****"
		}
	}
	return strings.Join(parts, " ")
}
