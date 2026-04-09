package discover

import (
	"encoding/json"
	"fmt"
	"os"
)

// ProjectConfig represents the structure of a .dbseer.json project configuration file.
// It allows a project to define multiple named database environments.
type ProjectConfig struct {
	// DefaultEnv is the name of the environment to use when none is specified.
	DefaultEnv string `json:"defaultEnv"`
	// Environments maps environment names to their configuration.
	Environments map[string]struct {
		// URL is the Postgres connection URL for this environment.
		URL string `json:"url"`
		// Readonly marks this environment as read-only when true.
		Readonly bool `json:"readonly,omitempty"`
	} `json:"environments"`
}

// ReadProjectConfig reads and parses a .dbseer.json file at path.
func ReadProjectConfig(path string) (*ProjectConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: reading %s: %w", path, err)
	}
	var cfg ProjectConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("config: parsing %s: %w", path, err)
	}
	return &cfg, nil
}

// PickEnvironment selects a URL from cfg based on the requested environment name.
// If requested is empty, the DefaultEnv is used. Returns an error if the named
// environment does not exist in cfg.
func PickEnvironment(cfg *ProjectConfig, requested string) (url string, err error) {
	name := requested
	if name == "" {
		name = cfg.DefaultEnv
	}
	if name == "" {
		return "", fmt.Errorf("config: no environment requested and no defaultEnv set")
	}
	env, ok := cfg.Environments[name]
	if !ok {
		return "", fmt.Errorf("config: environment %q not found", name)
	}
	if env.URL == "" {
		return "", fmt.Errorf("config: environment %q has no url", name)
	}
	return env.URL, nil
}
