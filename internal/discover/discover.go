package discover

import (
	"fmt"
	"os"
	"path/filepath"
)

// Options controls the behaviour of the Discover function.
type Options struct {
	// StartDir is the directory from which upward discovery begins.
	// Defaults to the current working directory if empty.
	StartDir string
	// PreferEnvironment names the environment to select from a .dbseer.json file.
	// If empty, the file's defaultEnv is used.
	PreferEnvironment string
}

// Discover walks upward from opts.StartDir, checking each directory for a
// Postgres connection URL using the following priority order at each level:
//
//  1. .env* files (DATABASE_URL / POSTGRES_URL / POSTGRES_URI / PG_URL)
//  2. prisma/schema.prisma
//  3. drizzle.config.{ts,js,mjs}
//  4. docker-compose.yml / compose.yaml
//  5. .dbseer.json
//
// The walk stops at the first match (first-hit-wins). If no source is found
// after reaching the filesystem root, Source{Kind: SourceNone} is returned
// without an error — the caller decides whether to fail or prompt.
func Discover(opts Options) (Source, error) {
	startDir := opts.StartDir
	if startDir == "" {
		var err error
		startDir, err = os.Getwd()
		if err != nil {
			return Source{}, fmt.Errorf("discover: getting working directory: %w", err)
		}
	}

	abs, err := filepath.Abs(startDir)
	if err != nil {
		return Source{}, fmt.Errorf("discover: resolving %s: %w", startDir, err)
	}

	// Verify the start directory exists.
	if _, err := os.Stat(abs); err != nil {
		return Source{}, fmt.Errorf("discover: start directory %s: %w", abs, err)
	}

	dir := abs
	for {
		src, err := checkDir(dir, opts.PreferEnvironment)
		if err != nil {
			return Source{}, err
		}
		if src.Kind != SourceNone {
			return src, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached the filesystem root without finding anything.
			break
		}

		// Stop if we can't read the parent.
		if _, err := os.Stat(parent); err != nil {
			break
		}

		dir = parent
	}

	return Source{Kind: SourceNone}, nil
}

// checkDir checks a single directory for all discovery sources in priority order.
// Returns Source{Kind: SourceNone} if nothing is found in that directory.
func checkDir(dir, preferEnv string) (Source, error) {
	// 1. .env* files
	envMap, foundFiles, err := ReadEnvFiles(dir)
	if err != nil {
		return Source{}, fmt.Errorf("discover: reading env files in %s: %w", dir, err)
	}
	if len(foundFiles) > 0 {
		if u, varName := FindEnvURL(envMap); u != "" {
			return Source{
				Kind:        SourceEnv,
				Path:        foundFiles[len(foundFiles)-1], // highest-priority file
				ProjectRoot: dir,
				URL:         u,
				EnvVar:      varName,
			}, nil
		}
	}

	// 2. prisma/schema.prisma
	schemaPath := filepath.Join(dir, "prisma", "schema.prisma")
	if _, err := os.Stat(schemaPath); err == nil {
		u, provider, envVar, parseErr := ParsePrismaDatasource(schemaPath)
		if parseErr == nil {
			// Reject non-postgres providers upstream.
			if provider != "" && provider != "postgresql" && provider != "postgres" {
				// Not a postgres datasource; skip.
			} else {
				if envVar != "" {
					// Resolve via .env chain at the prisma file's directory.
					prismaDir := filepath.Join(dir, "prisma")
					prismaEnv, _, _ := ReadEnvFiles(prismaDir)
					// Also merge parent dir env (lower priority).
					for k, v := range envMap {
						if _, exists := prismaEnv[k]; !exists {
							prismaEnv[k] = v
						}
					}
					u = LookupEnv(prismaEnv, envVar)
				}
				if u != "" {
					return Source{
						Kind:        SourcePrisma,
						Path:        schemaPath,
						ProjectRoot: dir,
						URL:         u,
						EnvVar:      envVar,
					}, nil
				}
			}
		}
	}

	// 3. drizzle.config.{ts,js,mjs}
	for _, name := range drizzleConfigNames {
		configPath := filepath.Join(dir, name)
		if _, err := os.Stat(configPath); err != nil {
			continue
		}
		u, envVar, parseErr := ParseDrizzleConfig(configPath)
		if parseErr != nil {
			// Warning: can't parse; move on.
			continue
		}
		if envVar != "" {
			u = LookupEnv(envMap, envVar)
		}
		if u != "" {
			return Source{
				Kind:        SourceDrizzle,
				Path:        configPath,
				ProjectRoot: dir,
				URL:         u,
				EnvVar:      envVar,
			}, nil
		}
	}

	// 4. docker-compose.yml / compose.yaml
	for _, name := range []string{"docker-compose.yml", "compose.yaml"} {
		composePath := filepath.Join(dir, name)
		if _, err := os.Stat(composePath); err != nil {
			continue
		}
		u, parseErr := ParseComposeFile(composePath)
		if parseErr != nil {
			continue
		}
		if u != "" {
			return Source{
				Kind:        SourceCompose,
				Path:        composePath,
				ProjectRoot: dir,
				URL:         u,
			}, nil
		}
	}

	// 5. .dbseer.json
	configPath := filepath.Join(dir, ".dbseer.json")
	if _, err := os.Stat(configPath); err == nil {
		cfg, readErr := ReadProjectConfig(configPath)
		if readErr == nil {
			u, pickErr := PickEnvironment(cfg, preferEnv)
			if pickErr == nil && u != "" {
				return Source{
					Kind:        SourceConfig,
					Path:        configPath,
					ProjectRoot: dir,
					URL:         u,
				}, nil
			}
		}
	}

	return Source{Kind: SourceNone}, nil
}
