package db

import (
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// Quote wraps one or more identifier parts using pgx.Identifier.Sanitize,
// producing a safely-quoted dot-joined identifier (e.g. "public"."users").
// Returns an error if any part contains a null byte.
func Quote(parts ...string) (string, error) {
	for _, p := range parts {
		if strings.ContainsRune(p, 0) {
			return "", fmt.Errorf("identifier contains null byte: %q", p)
		}
	}
	return pgx.Identifier(parts).Sanitize(), nil
}

// QualifiedTable returns "schema"."table" as a single sanitized string.
func QualifiedTable(schema, table string) (string, error) {
	return Quote(schema, table)
}

// QualifiedColumn returns "schema"."table"."col" as a single sanitized string.
func QualifiedColumn(schema, table, col string) (string, error) {
	return Quote(schema, table, col)
}
