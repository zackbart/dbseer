// Package discover implements the dbseer auto-discovery pipeline that walks
// upward from a start directory looking for a Postgres connection URL.
package discover

import (
	"fmt"
	"io"
	"net/url"
	"strings"
)

// SourceKind identifies the type of discovery source that produced a connection URL.
type SourceKind string

const (
	// SourceEnv indicates the URL came from a .env* file.
	SourceEnv SourceKind = "env"
	// SourceCompose indicates the URL was derived from a docker-compose.yml.
	SourceCompose SourceKind = "compose"
	// SourcePrisma indicates the URL came from prisma/schema.prisma.
	SourcePrisma SourceKind = "prisma"
	// SourceDrizzle indicates the URL came from a drizzle.config.{ts,js,mjs}.
	SourceDrizzle SourceKind = "drizzle"
	// SourceConfig indicates the URL came from a .dbseer.json project config.
	SourceConfig SourceKind = "dbseer-config"
	// SourceFlag indicates the URL was provided via a --url CLI flag.
	SourceFlag SourceKind = "flag"
	// SourceNone indicates no source was found.
	SourceNone SourceKind = "none"
)

// Source describes a resolved Postgres connection URL and how it was discovered.
type Source struct {
	// Kind identifies which discovery mechanism produced this source.
	Kind SourceKind
	// Path is the absolute path of the file that contained the URL.
	// Empty for SourceFlag and SourceNone.
	Path string
	// ProjectRoot is the directory containing the source file.
	// Used to derive the audit log path (.dbseer/history.jsonl).
	ProjectRoot string
	// URL is the resolved Postgres connection URL.
	URL string
	// EnvVar is the environment variable name if the URL was discovered via
	// an env() reference (Prisma, Drizzle) or a direct .env lookup.
	EnvVar string
}

// Render writes a human-readable --which summary to w.
// The password in the URL is redacted.
func (s Source) Render(w io.Writer) {
	if s.Kind == SourceNone {
		fmt.Fprintln(w, "source: none")
		fmt.Fprintln(w, "  no connection URL discovered")
		return
	}

	fmt.Fprintf(w, "source: %s\n", s.Kind)
	if s.Path != "" {
		fmt.Fprintf(w, "path:   %s\n", s.Path)
	}
	if s.EnvVar != "" {
		fmt.Fprintf(w, "env:    %s\n", s.EnvVar)
	}

	redacted := redactURL(s.URL)
	if redacted != "" {
		u, err := url.Parse(s.URL)
		if err == nil {
			fmt.Fprintf(w, "host:   %s\n", u.Hostname())
			dbName := strings.TrimPrefix(u.Path, "/")
			if dbName != "" {
				fmt.Fprintf(w, "database: %s\n", dbName)
			}
		}
		fmt.Fprintf(w, "url:    %s\n", redacted)
	}
}

// redactURL replaces the password in a Postgres URL with *****.
func redactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if u.User != nil {
		if _, hasPass := u.User.Password(); hasPass {
			u.User = url.UserPassword(u.User.Username(), "*****")
		}
	}
	return u.String()
}
