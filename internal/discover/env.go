package discover

import (
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

// envFileOrder defines the precedence order for .env files, from lowest to highest.
// Later entries in this list override earlier ones.
var envFileOrder = []string{
	".env",
	".env.development",
	".env.local",
	".env.development.local",
}

// envURLVars lists the environment variable names to check for a Postgres URL,
// in priority order (first found wins).
var envURLVars = []string{
	"DATABASE_URL",
	"POSTGRES_URL",
	"POSTGRES_URI",
	"PG_URL",
}

// ReadEnvFiles reads .env files in the given directory in precedence order
// (lowest to highest): .env, .env.development, .env.local, .env.development.local.
// Later files override earlier ones (higher precedence).
//
// Returns the merged map, the list of files actually found (in read order), and any error.
func ReadEnvFiles(dir string) (map[string]string, []string, error) {
	merged := make(map[string]string)
	var found []string

	for _, name := range envFileOrder {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		}
		m, err := godotenv.Read(path)
		if err != nil {
			return nil, nil, err
		}
		found = append(found, path)
		// Higher-priority files override lower-priority ones.
		for k, v := range m {
			merged[k] = v
		}
	}

	return merged, found, nil
}

// FindEnvURL looks up a Postgres connection URL in the provided env map.
// It checks DATABASE_URL, POSTGRES_URL, POSTGRES_URI, and PG_URL in that order.
// Returns the first found URL and the variable name. Returns empty strings if none found.
func FindEnvURL(envMap map[string]string) (url, varName string) {
	for _, key := range envURLVars {
		if v, ok := envMap[key]; ok && v != "" {
			return v, key
		}
	}
	return "", ""
}

// LookupEnv returns the value of name from envMap, falling back to os.Getenv
// if the key is not present in the map. Used by the Prisma parser to resolve
// env("VAR") references against the .env chain.
func LookupEnv(envMap map[string]string, name string) string {
	if v, ok := envMap[name]; ok {
		return v
	}
	return os.Getenv(name)
}
