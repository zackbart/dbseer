package discover

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var nestedScanSkipDirs = map[string]struct{}{
	".git":         {},
	".next":        {},
	".turbo":       {},
	".venv":        {},
	"build":        {},
	"coverage":     {},
	"dist":         {},
	"node_modules": {},
	"tmp":          {},
	"vendor":       {},
}

// DiscoverNested walks downward from opts.StartDir and returns one discovery
// candidate per directory, using the same priority rules as Discover.
//
// The walk is intentionally forgiving: unreadable directories and malformed
// nested config/env files are skipped so a single bad subtree doesn't block
// picking another valid project.
func DiscoverNested(opts Options) ([]Source, error) {
	startDir := opts.StartDir
	if startDir == "" {
		var err error
		startDir, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	}

	abs, err := filepath.Abs(startDir)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(abs); err != nil {
		return nil, err
	}

	var out []Source
	seen := make(map[string]struct{})

	err = filepath.WalkDir(abs, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		if path != abs {
			if _, skip := nestedScanSkipDirs[d.Name()]; skip {
				return fs.SkipDir
			}
		}

		src, err := checkDir(path, opts.PreferEnvironment)
		if err != nil || src.Kind == SourceNone {
			return nil
		}

		key := sourceKey(src)
		if _, exists := seen[key]; exists {
			return nil
		}
		seen[key] = struct{}{}
		out = append(out, src)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(out, func(i, j int) bool {
		leftDepth := pathDepth(abs, out[i].Path)
		rightDepth := pathDepth(abs, out[j].Path)
		if leftDepth != rightDepth {
			return leftDepth < rightDepth
		}
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Kind < out[j].Kind
	})

	return out, nil
}

func sourceKey(src Source) string {
	return strings.Join([]string{
		string(src.Kind),
		src.Path,
		src.ProjectRoot,
		src.URL,
		src.EnvVar,
	}, "\x00")
}

func pathDepth(base, path string) int {
	if path == "" {
		return 0
	}
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return strings.Count(path, string(os.PathSeparator))
	}
	if rel == "." {
		return 0
	}
	return strings.Count(rel, string(os.PathSeparator)) + 1
}
