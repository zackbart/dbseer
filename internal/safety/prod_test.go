package safety

import "testing"

func TestIsProdHost(t *testing.T) {
	cases := []struct {
		host        string
		wantIsProd  bool
		wantReason  string // substring that should appear in reason
	}{
		// Known provider suffixes.
		{"mydb.us-east-1.rds.amazonaws.com", true, "rds.amazonaws.com"},
		{"db.abcxyz.supabase.co", true, "supabase.co"},
		{"db.abcxyz.supabase.com", true, "supabase.com"},
		{"ep-quiet-moon-123.us-east-2.aws.neon.tech", true, "neon.tech"},
		{"ep-quiet-moon-123.us-east-2.aws.neon.build", true, "neon.build"},
		{"mydb.planetscale.com", true, "planetscale.com"},
		{"free-tier.cockroachlabs.cloud", true, "cockroachlabs.cloud"},

		// "prod" as a whole word in a segment.
		{"db-prod-1.internal", true, "prod"},
		{"prod.example.com", true, "prod"},
		{"my_prod_db", true, "prod"},

		// "prod" NOT as a whole word — must NOT match.
		{"reproducible-host.example.com", false, ""},
		{"productdb.example.com", false, ""},
		{"production-ready.example.com", false, ""},

		// Clearly dev hosts.
		{"localhost", false, ""},
		{"127.0.0.1", false, ""},
		{"db.dev.example.com", false, ""},
		{"staging-db.example.com", false, ""},
	}

	for _, tc := range cases {
		isProd, reason := IsProdHost(tc.host)
		if isProd != tc.wantIsProd {
			t.Errorf("IsProdHost(%q) isProd = %v, want %v", tc.host, isProd, tc.wantIsProd)
		}
		if tc.wantIsProd && tc.wantReason != "" && !contains(reason, tc.wantReason) {
			t.Errorf("IsProdHost(%q) reason = %q, want it to contain %q", tc.host, reason, tc.wantReason)
		}
		if !tc.wantIsProd && reason != "" {
			t.Errorf("IsProdHost(%q) reason = %q, want empty for non-prod", tc.host, reason)
		}
	}
}
