package safety

import (
	"regexp"
	"strings"
)

// prodSuffixes lists known managed-Postgres provider hostname suffixes that
// strongly indicate a production (or at least non-local) database.
var prodSuffixes = []struct {
	suffix string
	reason string
}{
	{".rds.amazonaws.com", "matches *.rds.amazonaws.com"},
	{".supabase.co", "matches *.supabase.co"},
	{".supabase.com", "matches *.supabase.com"},
	{".neon.tech", "matches *.neon.tech"},
	{".neon.build", "matches *.neon.build"},
	{".planetscale.com", "matches *.planetscale.com"},
	{".cockroachlabs.cloud", "matches *.cockroachlabs.cloud"},
}

// prodWordRe matches the word "prod" as a whole word when the host is split on
// dot, hyphen, or underscore delimiters. This prevents false positives such as
// "reproducible" or "productdb".
var prodWordRe = regexp.MustCompile(`(?i)^prod$`)

// IsProdHost reports whether the given hostname looks like a production host.
// It returns (true, reason) on a match and (false, "") otherwise.
//
// Detection rules:
//   - Hostname suffix matches a known managed-Postgres provider.
//   - A segment of the hostname (split on '.', '-', '_') is exactly "prod"
//     (case-insensitive).
func IsProdHost(host string) (bool, string) {
	lower := strings.ToLower(host)

	for _, ps := range prodSuffixes {
		if strings.HasSuffix(lower, ps.suffix) {
			return true, ps.reason
		}
	}

	// Split on word-boundary delimiters and check each segment.
	segments := strings.FieldsFunc(lower, func(r rune) bool {
		return r == '.' || r == '-' || r == '_'
	})
	for _, seg := range segments {
		if prodWordRe.MatchString(seg) {
			return true, "hostname contains 'prod'"
		}
	}

	return false, ""
}
