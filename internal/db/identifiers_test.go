package db

import (
	"strings"
	"testing"
)

func TestQuote(t *testing.T) {
	t.Run("single identifier", func(t *testing.T) {
		got, err := Quote("users")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != `"users"` {
			t.Errorf("got %q, want %q", got, `"users"`)
		}
	})

	t.Run("schema-qualified", func(t *testing.T) {
		got, err := Quote("public", "users")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != `"public"."users"` {
			t.Errorf("got %q, want %q", got, `"public"."users"`)
		}
	})

	t.Run("special characters escaped", func(t *testing.T) {
		got, err := Quote(`weird"name`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// pgx.Identifier.Sanitize escapes double quotes by doubling them.
		if !strings.Contains(got, `""`) {
			t.Errorf("expected escaped double quote in %q", got)
		}
	})

	t.Run("null byte rejected", func(t *testing.T) {
		_, err := Quote("bad\x00name")
		if err == nil {
			t.Fatal("expected error for null byte, got nil")
		}
	})
}

func TestQualifiedTable(t *testing.T) {
	got, err := QualifiedTable("public", "orders")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `"public"."orders"`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestQualifiedColumn(t *testing.T) {
	got, err := QualifiedColumn("public", "orders", "created_at")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `"public"."orders"."created_at"`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
