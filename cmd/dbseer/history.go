package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/zackbart/dbseer/internal/discover"
	"github.com/zackbart/dbseer/internal/safety"
)

// runHistory implements the "dbseer history" subcommand.
// It reads .dbseer/history.jsonl from the discovered project root (or
// $XDG_STATE_HOME/dbseer/ if run outside a project) and prints a
// human-readable table to stdout.
//
// Usage: dbseer history [--limit N] [--since DURATION] [--table schema.name] [--json]
func runHistory(args []string) error {
	fs := flag.NewFlagSet("dbseer history", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: dbseer history [flags]\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		fs.PrintDefaults()
	}

	var (
		limitFlag  int
		sinceFlag  string
		tableFlag  string
		jsonFlag   bool
	)

	fs.IntVar(&limitFlag, "limit", 50, "Maximum number of entries to return")
	fs.StringVar(&sinceFlag, "since", "", `Filter entries newer than this duration (e.g. "1h", "24h", "7d")`)
	fs.StringVar(&tableFlag, "table", "", `Filter by table name ("schema.name")`)
	fs.BoolVar(&jsonFlag, "json", false, "Output raw JSONL instead of formatted table")

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Parse --since duration (support "7d" style as well as standard durations).
	var sinceTS time.Time
	if sinceFlag != "" {
		d, err := parseDuration(sinceFlag)
		if err != nil {
			return fmt.Errorf("--since: %w", err)
		}
		sinceTS = time.Now().Add(-d)
	}

	// Discover project root for log dir resolution.
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	source, err := discover.Discover(discover.Options{StartDir: cwd})
	if err != nil {
		// Non-fatal: fall back to XDG state dir.
		source = discover.Source{}
	}

	logDir := safety.ResolveLogDir(source.ProjectRoot)
	logger, err := safety.NewLogger(logDir)
	if err != nil {
		return fmt.Errorf("opening audit log at %s: %w", logDir, err)
	}

	entries, err := logger.Read(limitFlag, sinceTS, tableFlag)
	if err != nil {
		return fmt.Errorf("reading audit log: %w", err)
	}

	if len(entries) == 0 {
		fmt.Fprintln(os.Stdout, "no history entries found")
		return nil
	}

	if jsonFlag {
		enc := json.NewEncoder(os.Stdout)
		for _, e := range entries {
			if err := enc.Encode(e); err != nil {
				return fmt.Errorf("encoding entry: %w", err)
			}
		}
		return nil
	}

	// Formatted table output.
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TS\tOP\tTABLE\tAFFECTED\tSQL")
	fmt.Fprintln(w, strings.Repeat("-", 24)+"\t"+strings.Repeat("-", 6)+"\t"+strings.Repeat("-", 24)+"\t"+strings.Repeat("-", 8)+"\t"+strings.Repeat("-", 40))

	for _, e := range entries {
		ts := e.TS.Format(time.RFC3339)
		sqlSnip := e.SQL
		if len(sqlSnip) > 80 {
			sqlSnip = sqlSnip[:77] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n",
			ts,
			e.Op,
			e.Table,
			e.Affected,
			sqlSnip,
		)
	}

	return w.Flush()
}

// parseDuration extends time.ParseDuration to support "7d", "30d" etc.
// Days are converted to hours (1d = 24h).
func parseDuration(s string) (time.Duration, error) {
	if strings.HasSuffix(s, "d") {
		numStr := strings.TrimSuffix(s, "d")
		var days int
		if _, err := fmt.Sscanf(numStr, "%d", &days); err != nil {
			return 0, fmt.Errorf("invalid duration %q: %w", s, err)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}
