package discover

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type composeFile struct {
	Services map[string]composeService `yaml:"services"`
}

type composeService struct {
	Image       string   `yaml:"image"`
	Environment any      `yaml:"environment"` // list or map
	Ports       []string `yaml:"ports"`
}

// postgresImages lists image name substrings that indicate a Postgres-compatible service.
var postgresImages = []string{
	"postgres",
	"timescale/timescaledb",
	"pgvector",
	"ankane/pgvector",
	"postgis",
}

// isPostgresImage returns true if the image name refers to a Postgres-compatible image.
func isPostgresImage(image string) bool {
	// Strip tag suffix for matching.
	name := image
	if idx := strings.Index(image, ":"); idx >= 0 {
		name = image[:idx]
	}
	for _, pat := range postgresImages {
		if strings.Contains(name, pat) {
			return true
		}
	}
	return false
}

// ParseComposeFile parses a docker-compose YAML file and returns a Postgres
// connection URL derived from the first Postgres-like service found.
//
// Defaults: user=postgres, password=postgres, db=postgres, port=5432.
func ParseComposeFile(path string) (url string, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("compose: reading %s: %w", path, err)
	}

	var cf composeFile
	if err := yaml.Unmarshal(data, &cf); err != nil {
		return "", fmt.Errorf("compose: parsing %s: %w", path, err)
	}

	for _, svc := range cf.Services {
		if !isPostgresImage(svc.Image) {
			continue
		}

		envMap := parseComposeEnvironment(svc.Environment)

		user := envValue(envMap, "POSTGRES_USER", "postgres")
		password := envValue(envMap, "POSTGRES_PASSWORD", "postgres")
		db := envValue(envMap, "POSTGRES_DB", user)
		port := findHostPort(svc.Ports, "5432")

		return fmt.Sprintf("postgres://%s:%s@127.0.0.1:%s/%s?sslmode=disable",
			user, password, port, db), nil
	}

	return "", fmt.Errorf("compose: no postgres service found in %s", path)
}

// parseComposeEnvironment normalises the compose environment field which can be
// either a map[string]string or a list of "KEY=VALUE" strings.
func parseComposeEnvironment(env any) map[string]string {
	result := make(map[string]string)
	if env == nil {
		return result
	}

	switch v := env.(type) {
	case map[string]any:
		for key, val := range v {
			if val == nil {
				result[key] = ""
			} else {
				result[key] = fmt.Sprintf("%v", val)
			}
		}
	case []any:
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				continue
			}
			idx := strings.Index(s, "=")
			if idx < 0 {
				result[s] = ""
				continue
			}
			result[s[:idx]] = s[idx+1:]
		}
	}
	return result
}

// envValue returns the value for key in m, or def if not found or empty.
func envValue(m map[string]string, key, def string) string {
	if v, ok := m[key]; ok && v != "" {
		return v
	}
	return def
}

// findHostPort finds the host-side port for a given target port in a list of
// port mappings (e.g., "5433:5432"). Returns def if not found.
func findHostPort(ports []string, target string) string {
	for _, p := range ports {
		parts := strings.SplitN(p, ":", 2)
		var hostPort, containerPort string
		switch len(parts) {
		case 1:
			hostPort = parts[0]
			containerPort = parts[0]
		case 2:
			hostPort = parts[0]
			containerPort = parts[1]
		}
		// Strip any IP prefix from host port (e.g., "0.0.0.0:5433").
		if idx := strings.LastIndex(hostPort, ":"); idx >= 0 {
			hostPort = hostPort[idx+1:]
		}
		if containerPort == target {
			if hostPort != "" {
				if _, err := strconv.Atoi(hostPort); err == nil {
					return hostPort
				}
			}
			return target
		}
	}
	return target
}
