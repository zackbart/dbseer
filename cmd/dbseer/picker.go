package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/zackbart/dbseer/internal/discover"
)

func resolveSource(startDir, preferEnv string, in io.Reader, out io.Writer) (discover.Source, error) {
	primary, err := discover.Discover(discover.Options{
		StartDir:          startDir,
		PreferEnvironment: preferEnv,
	})
	if err != nil {
		return discover.Source{}, err
	}

	candidates, err := discover.DiscoverNested(discover.Options{
		StartDir:          startDir,
		PreferEnvironment: preferEnv,
	})
	if err != nil {
		return discover.Source{}, err
	}
	candidates = mergeSources(primary, candidates)

	switch {
	case primary.Kind != discover.SourceNone && len(candidates) <= 1:
		return primary, nil
	case primary.Kind == discover.SourceNone && len(candidates) == 0:
		return primary, nil
	case primary.Kind == discover.SourceNone && len(candidates) == 1:
		return candidates[0], nil
	}

	if !stdioIsInteractive() {
		if primary.Kind != discover.SourceNone {
			return primary, nil
		}
		return discover.Source{}, fmt.Errorf(
			"found %d database sources under %s; rerun in a terminal to choose one, or cd into the app you want",
			len(candidates),
			startDir,
		)
	}

	return promptForSource(startDir, candidates, primary, in, out)
}

func mergeSources(primary discover.Source, candidates []discover.Source) []discover.Source {
	var merged []discover.Source
	seen := make(map[string]struct{})

	if primary.Kind != discover.SourceNone {
		key := sourceSignature(primary)
		seen[key] = struct{}{}
		merged = append(merged, primary)
	}

	for _, candidate := range candidates {
		key := sourceSignature(candidate)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		merged = append(merged, candidate)
	}

	return merged
}

func promptForSource(startDir string, candidates []discover.Source, primary discover.Source, in io.Reader, out io.Writer) (discover.Source, error) {
	defaultIndex := 0
	if primary.Kind != discover.SourceNone {
		for i, candidate := range candidates {
			if sourceSignature(candidate) == sourceSignature(primary) {
				defaultIndex = i
				break
			}
		}
	}

	reader := bufio.NewReader(in)
	fmt.Fprintln(out, "Multiple database sources found:")
	for i, candidate := range candidates {
		suffix := ""
		if i == defaultIndex {
			suffix = " (default)"
		}
		fmt.Fprintf(out, "  %d. %s%s\n", i+1, formatSourceOption(startDir, candidate), suffix)
	}

	for {
		fmt.Fprintf(out, "Select a source [%d]: ", defaultIndex+1)
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return discover.Source{}, err
		}

		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			return candidates[defaultIndex], nil
		}
		if strings.EqualFold(trimmed, "q") || strings.EqualFold(trimmed, "quit") {
			return discover.Source{}, fmt.Errorf("source selection canceled")
		}

		choice, convErr := strconv.Atoi(trimmed)
		if convErr == nil && choice >= 1 && choice <= len(candidates) {
			return candidates[choice-1], nil
		}

		fmt.Fprintf(out, "Enter a number between 1 and %d, or q to cancel.\n", len(candidates))
		if errors.Is(err, io.EOF) {
			return candidates[defaultIndex], nil
		}
	}
}

func formatSourceOption(startDir string, src discover.Source) string {
	parts := []string{string(src.Kind)}

	if src.Path != "" {
		parts = append(parts, shortenPath(startDir, src.Path))
	}
	if src.EnvVar != "" {
		parts = append(parts, src.EnvVar)
	}
	if location := describeURL(src.URL); location != "" {
		parts = append(parts, location)
	}

	return strings.Join(parts, "  ")
}

func shortenPath(startDir, path string) string {
	rel, err := filepath.Rel(startDir, path)
	if err == nil && rel != "" && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return path
}

func describeURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}

	host := u.Hostname()
	database := strings.TrimPrefix(u.Path, "/")
	switch {
	case host != "" && database != "":
		return host + "/" + database
	case host != "":
		return host
	case database != "":
		return database
	default:
		return ""
	}
}

func stdioIsInteractive() bool {
	return fileIsInteractive(os.Stdin) && fileIsInteractive(os.Stdout)
}

func fileIsInteractive(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func sourceSignature(src discover.Source) string {
	return strings.Join([]string{
		string(src.Kind),
		src.Path,
		src.ProjectRoot,
		src.URL,
		src.EnvVar,
	}, "\x00")
}
