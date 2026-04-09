package safety

import (
	"os"
	"testing"
	"time"
)

func TestLogger_AppendAndRead(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	entries := []Entry{
		{TS: base, Op: "INSERT", Table: "public.users", Affected: 1, SQL: "INSERT INTO users ...", User: "alice", Database: "dev"},
		{TS: base.Add(time.Minute), Op: "UPDATE", Table: "public.orders", Affected: 2, SQL: "UPDATE orders ...", User: "bob", Database: "dev"},
		{TS: base.Add(2 * time.Minute), Op: "DELETE", Table: "public.users", Affected: 1, SQL: "DELETE FROM users ...", User: "alice", Database: "dev"},
	}

	for _, e := range entries {
		if err := logger.Append(e); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}

	// Read all, no filter.
	all, err := logger.Read(0, time.Time{}, "")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("Read() returned %d entries, want 3", len(all))
	}
	// Newest first.
	if all[0].Op != "DELETE" {
		t.Errorf("first entry should be DELETE (newest), got %q", all[0].Op)
	}

	// Filter by table.
	filtered, err := logger.Read(0, time.Time{}, "public.users")
	if err != nil {
		t.Fatalf("Read (table filter): %v", err)
	}
	if len(filtered) != 2 {
		t.Fatalf("Read(table=public.users) returned %d entries, want 2", len(filtered))
	}

	// Filter by sinceTS — only entries after the first.
	since, err := logger.Read(0, base.Add(30*time.Second), "")
	if err != nil {
		t.Fatalf("Read (sinceTS): %v", err)
	}
	if len(since) != 2 {
		t.Fatalf("Read(sinceTS) returned %d entries, want 2", len(since))
	}

	// Limit.
	limited, err := logger.Read(1, time.Time{}, "")
	if err != nil {
		t.Fatalf("Read (limit): %v", err)
	}
	if len(limited) != 1 {
		t.Fatalf("Read(limit=1) returned %d entries, want 1", len(limited))
	}
}

func TestLogger_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	entries, err := logger.Read(10, time.Time{}, "")
	if err != nil {
		t.Fatalf("Read on missing file: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for empty log, got %d", len(entries))
	}
}

func TestResolveLogDir(t *testing.T) {
	dir := ResolveLogDir("/myproject")
	if dir != "/myproject/.dbseer" {
		t.Errorf("ResolveLogDir with root = %q, want %q", dir, "/myproject/.dbseer")
	}

	// Without project root, should use XDG or home fallback.
	os.Setenv("XDG_STATE_HOME", "/custom/state")
	dir = ResolveLogDir("")
	if dir != "/custom/state/dbseer" {
		t.Errorf("ResolveLogDir with XDG_STATE_HOME = %q, want %q", dir, "/custom/state/dbseer")
	}
	os.Unsetenv("XDG_STATE_HOME")
}
