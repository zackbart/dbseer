package discover

import (
	"fmt"
	"os"
	"regexp"
)

var (
	// Literal string value: url: "..." or connectionString: "..."
	drizzleLiteralRe = regexp.MustCompile(`(?:url|connectionString)\s*:\s*"([^"]+)"`)

	// process.env.VAR or process.env["VAR"]
	drizzleProcessEnvRe = regexp.MustCompile(`(?:url|connectionString)\s*:\s*process\.env(?:\.([A-Za-z_][A-Za-z0-9_]*)|(?:\["([^"]+)"\]))`)
)

// drizzleConfigNames lists the candidate filenames to check for a Drizzle config.
var drizzleConfigNames = []string{
	"drizzle.config.ts",
	"drizzle.config.js",
	"drizzle.config.mjs",
}

// ParseDrizzleConfig reads a Drizzle config file and extracts a Postgres connection
// URL or an environment variable reference.
//
// If the URL is a literal, it is returned in url and envVar is empty.
// If the config uses process.env.VAR, envVar is populated and url is empty — the
// caller should resolve via LookupEnv.
//
// Returns an error if the file cannot be read or no recognizable URL pattern is found.
func ParseDrizzleConfig(configPath string) (url, envVar string, err error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return "", "", fmt.Errorf("drizzle: reading %s: %w", configPath, err)
	}

	cleaned := stripLineComments(string(data))

	// Check for process.env references first (more specific).
	if m := drizzleProcessEnvRe.FindStringSubmatch(cleaned); m != nil {
		varName := m[1]
		if varName == "" {
			varName = m[2]
		}
		if varName != "" {
			return "", varName, nil
		}
	}

	// Check for literal string.
	if m := drizzleLiteralRe.FindStringSubmatch(cleaned); m != nil {
		return m[1], "", nil
	}

	return "", "", fmt.Errorf("drizzle: no recognizable url or connectionString in %s", configPath)
}
