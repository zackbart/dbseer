package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zackbart/dbseer/internal/discover"
)

func TestMergeSources_DedupesPrimary(t *testing.T) {
	primary := discover.Source{
		Kind:        discover.SourceEnv,
		Path:        "/repo/apps/api/.env",
		ProjectRoot: "/repo/apps/api",
		URL:         "postgres://localhost/api",
		EnvVar:      "DATABASE_URL",
	}

	merged := mergeSources(primary, []discover.Source{
		primary,
		{
			Kind:        discover.SourceEnv,
			Path:        "/repo/apps/worker/.env",
			ProjectRoot: "/repo/apps/worker",
			URL:         "postgres://localhost/worker",
			EnvVar:      "DATABASE_URL",
		},
	})

	if len(merged) != 2 {
		t.Fatalf("got %d merged sources, want 2", len(merged))
	}
	if merged[0].Path != primary.Path {
		t.Fatalf("got primary %q, want %q", merged[0].Path, primary.Path)
	}
}

func TestPromptForSource_SelectsChoice(t *testing.T) {
	startDir := "/repo"
	candidates := []discover.Source{
		{
			Kind:        discover.SourceEnv,
			Path:        "/repo/apps/api/.env",
			ProjectRoot: "/repo/apps/api",
			URL:         "postgres://localhost/api",
			EnvVar:      "DATABASE_URL",
		},
		{
			Kind:        discover.SourceEnv,
			Path:        "/repo/apps/worker/.env",
			ProjectRoot: "/repo/apps/worker",
			URL:         "postgres://localhost/worker",
			EnvVar:      "DATABASE_URL",
		},
	}

	var out bytes.Buffer
	selected, err := promptForSource(startDir, candidates, candidates[0], strings.NewReader("2\n"), &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if selected.Path != candidates[1].Path {
		t.Fatalf("got %q, want %q", selected.Path, candidates[1].Path)
	}
	if !strings.Contains(out.String(), "apps/api/.env") {
		t.Fatalf("prompt output missing relative path: %s", out.String())
	}
}

func TestPromptForSource_EmptyInputUsesDefault(t *testing.T) {
	candidates := []discover.Source{
		{
			Kind:        discover.SourceEnv,
			Path:        "/repo/apps/api/.env",
			ProjectRoot: "/repo/apps/api",
			URL:         "postgres://localhost/api",
			EnvVar:      "DATABASE_URL",
		},
		{
			Kind:        discover.SourceEnv,
			Path:        "/repo/apps/worker/.env",
			ProjectRoot: "/repo/apps/worker",
			URL:         "postgres://localhost/worker",
			EnvVar:      "DATABASE_URL",
		},
	}

	selected, err := promptForSource("/repo", candidates, candidates[1], strings.NewReader("\n"), &bytes.Buffer{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if selected.Path != candidates[1].Path {
		t.Fatalf("got %q, want %q", selected.Path, candidates[1].Path)
	}
}

func TestFormatSourceOption_ShowsRelativePathAndDB(t *testing.T) {
	option := formatSourceOption("/repo", discover.Source{
		Kind:        discover.SourceEnv,
		Path:        filepath.Join("/repo", "apps", "api", ".env"),
		ProjectRoot: filepath.Join("/repo", "apps", "api"),
		URL:         "postgres://localhost:5432/appdb",
		EnvVar:      "DATABASE_URL",
	})

	if !strings.Contains(option, "apps/api/.env") {
		t.Fatalf("option %q missing relative path", option)
	}
	if !strings.Contains(option, "localhost/appdb") {
		t.Fatalf("option %q missing host/database", option)
	}
}
