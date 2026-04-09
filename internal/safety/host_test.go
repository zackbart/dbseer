package safety

import (
	"testing"
)

func TestParse_URLForm(t *testing.T) {
	info, err := Parse("postgres://user:pass@host.example.com:5432/mydb")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Host != "host.example.com" {
		t.Errorf("Host = %q, want %q", info.Host, "host.example.com")
	}
	if info.Port != "5432" {
		t.Errorf("Port = %q, want %q", info.Port, "5432")
	}
	if info.Database != "mydb" {
		t.Errorf("Database = %q, want %q", info.Database, "mydb")
	}
	if info.User != "user" {
		t.Errorf("User = %q, want %q", info.User, "user")
	}
	if info.IsLocalhost {
		t.Error("IsLocalhost should be false for remote host")
	}
}

func TestParse_KeywordForm(t *testing.T) {
	info, err := Parse("host=localhost port=5432 dbname=app user=pguser")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Host != "localhost" {
		t.Errorf("Host = %q, want %q", info.Host, "localhost")
	}
	if info.Port != "5432" {
		t.Errorf("Port = %q, want %q", info.Port, "5432")
	}
	if info.Database != "app" {
		t.Errorf("Database = %q, want %q", info.Database, "app")
	}
	if info.User != "pguser" {
		t.Errorf("User = %q, want %q", info.User, "pguser")
	}
	if !info.IsLocalhost {
		t.Error("IsLocalhost should be true for localhost")
	}
}

func TestParse_Invalid(t *testing.T) {
	_, err := Parse("not a valid dsn ://\x00")
	if err == nil {
		t.Error("expected error for invalid DSN, got nil")
	}
}

func TestIsLocalhost(t *testing.T) {
	cases := []struct {
		host string
		want bool
	}{
		{"localhost", true},
		{"127.0.0.1", true},
		{"::1", true},
		{"0.0.0.0", true},
		{"", true},          // unix socket
		{"myhost.local", true},
		{"db.local", true},
		{"192.168.1.1", false},
		{"db.example.com", false},
		{"10.0.0.1", false},
		{"prod.db.example.com", false},
	}
	for _, tc := range cases {
		got := isLocalhost(tc.host)
		if got != tc.want {
			t.Errorf("isLocalhost(%q) = %v, want %v", tc.host, got, tc.want)
		}
	}
}

func TestRedact_URL(t *testing.T) {
	info := URLInfo{Raw: "postgres://alice:s3cr3t@localhost:5432/dev"}
	redacted := info.Redact()
	if contains(redacted, "s3cr3t") {
		t.Errorf("Redact() still contains password: %q", redacted)
	}
	if !contains(redacted, "alice") {
		t.Errorf("Redact() should retain username, got: %q", redacted)
	}
}

func TestRedact_Keyword(t *testing.T) {
	info := URLInfo{Raw: "host=localhost password=secret dbname=dev"}
	redacted := info.Redact()
	if contains(redacted, "secret") {
		t.Errorf("Redact() still contains password: %q", redacted)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}
