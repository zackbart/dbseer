package safety

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const historyFile = "history.jsonl"

// Entry is a single audit log record written to the history file for every
// successful mutation executed by dbseer.
type Entry struct {
	// TS is the UTC timestamp of the operation.
	TS time.Time `json:"ts"`
	// Op is the SQL operation type: "INSERT", "UPDATE", or "DELETE".
	Op string `json:"op"`
	// Table is the fully-qualified table name ("schema.table").
	Table string `json:"table"`
	// Affected is the number of rows affected by the operation.
	Affected int64 `json:"affected"`
	// SQL is the parameterized SQL statement that was executed.
	SQL string `json:"sql"`
	// Params are the query parameters bound to the SQL statement.
	Params []any `json:"params,omitempty"`
	// User is the Postgres username that performed the operation.
	User string `json:"user,omitempty"`
	// Database is the Postgres database name where the operation occurred.
	Database string `json:"database,omitempty"`
}

// Logger appends Entry values to a JSONL file and supports reading them back.
type Logger struct {
	path string
	mu   sync.Mutex
}

// NewLogger creates a Logger that writes to <dir>/history.jsonl. The directory
// is created if it does not already exist.
func NewLogger(dir string) (*Logger, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("safety: create log dir %q: %w", dir, err)
	}
	return &Logger{path: filepath.Join(dir, historyFile)}, nil
}

// Append serialises e as a single JSON line and appends it to the log file.
// The call is goroutine-safe.
func (l *Logger) Append(e Entry) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o640)
	if err != nil {
		return fmt.Errorf("safety: open log %q: %w", l.path, err)
	}
	defer f.Close()

	data, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("safety: marshal entry: %w", err)
	}
	data = append(data, '\n')

	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("safety: write log: %w", err)
	}
	// Flush to OS via Sync (best-effort durability).
	_ = f.Sync()
	return nil
}

// Read returns up to limit entries from the log, newest first. Pass 0 for
// limit to return all entries. sinceTS and table are optional filters; their
// zero values mean "no filter". For v0.1 the whole file is read into memory;
// optimise in v0.2.
func (l *Logger) Read(limit int, sinceTS time.Time, table string) ([]Entry, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	f, err := os.Open(l.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("safety: open log %q: %w", l.path, err)
	}
	defer f.Close()

	var all []Entry
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var e Entry
		if err := json.Unmarshal(line, &e); err != nil {
			// Skip malformed lines (log rotation artefacts, etc.).
			continue
		}
		if !sinceTS.IsZero() && e.TS.Before(sinceTS) {
			continue
		}
		if table != "" && e.Table != table {
			continue
		}
		all = append(all, e)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("safety: read log: %w", err)
	}

	// Reverse for newest-first.
	for i, j := 0, len(all)-1; i < j; i, j = i+1, j-1 {
		all[i], all[j] = all[j], all[i]
	}

	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}

// ResolveLogDir returns the directory that should be used for the audit log.
// If projectRoot is non-empty it returns <projectRoot>/.dbseer/. Otherwise it
// falls back to $XDG_STATE_HOME/dbseer/ (defaulting to
// $HOME/.local/state/dbseer/ when XDG_STATE_HOME is unset).
func ResolveLogDir(projectRoot string) string {
	if projectRoot != "" {
		return filepath.Join(projectRoot, ".dbseer")
	}
	stateHome := os.Getenv("XDG_STATE_HOME")
	if stateHome == "" {
		home, _ := os.UserHomeDir()
		stateHome = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(stateHome, "dbseer")
}
