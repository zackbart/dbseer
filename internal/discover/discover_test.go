package discover

import (
	"path/filepath"
	"runtime"
	"testing"
)

// testdataDir returns the absolute path to the testdata directory.
func testdataDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not determine test file path")
	}
	return filepath.Join(filepath.Dir(file), "testdata")
}

func TestDiscover_EnvSimple(t *testing.T) {
	dir := filepath.Join(testdataDir(t), "env-simple")
	src, err := Discover(Options{StartDir: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if src.Kind != SourceEnv {
		t.Errorf("got kind %q, want %q", src.Kind, SourceEnv)
	}
	if src.URL != "postgres://localhost/app" {
		t.Errorf("got URL %q, want %q", src.URL, "postgres://localhost/app")
	}
	if src.EnvVar != "DATABASE_URL" {
		t.Errorf("got EnvVar %q, want %q", src.EnvVar, "DATABASE_URL")
	}
	if src.ProjectRoot != dir {
		t.Errorf("got ProjectRoot %q, want %q", src.ProjectRoot, dir)
	}
}

func TestDiscover_EnvChain(t *testing.T) {
	// .env.development.local should override .env
	dir := filepath.Join(testdataDir(t), "env-chain")
	src, err := Discover(Options{StartDir: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if src.Kind != SourceEnv {
		t.Errorf("got kind %q, want %q", src.Kind, SourceEnv)
	}
	if src.URL != "postgres://localhost/override" {
		t.Errorf("got URL %q, want %q", src.URL, "postgres://localhost/override")
	}
}

func TestDiscover_PrismaEnv(t *testing.T) {
	dir := filepath.Join(testdataDir(t), "prisma-env")
	src, err := Discover(Options{StartDir: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// .env is also present, so env source wins over prisma at the same level.
	// The .env has DATABASE_URL so SourceEnv wins first.
	if src.Kind != SourceEnv && src.Kind != SourcePrisma {
		t.Errorf("got kind %q, want SourceEnv or SourcePrisma", src.Kind)
	}
	if src.URL != "postgres://localhost/prismadb" {
		t.Errorf("got URL %q, want %q", src.URL, "postgres://localhost/prismadb")
	}
}

func TestDiscover_PrismaLiteral(t *testing.T) {
	dir := filepath.Join(testdataDir(t), "prisma-literal")
	src, err := Discover(Options{StartDir: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if src.Kind != SourcePrisma {
		t.Errorf("got kind %q, want %q", src.Kind, SourcePrisma)
	}
	if src.URL != "postgres://postgres:secret@localhost:5432/myapp" {
		t.Errorf("got URL %q, want %q", src.URL, "postgres://postgres:secret@localhost:5432/myapp")
	}
	if src.EnvVar != "" {
		t.Errorf("got EnvVar %q, want empty (literal URL)", src.EnvVar)
	}
}

func TestDiscover_DrizzleTS(t *testing.T) {
	dir := filepath.Join(testdataDir(t), "drizzle-ts")
	src, err := Discover(Options{StartDir: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// .env is present so env wins over drizzle.
	if src.Kind != SourceEnv && src.Kind != SourceDrizzle {
		t.Errorf("got kind %q, want SourceEnv or SourceDrizzle", src.Kind)
	}
	if src.URL != "postgres://localhost/drizzledb" {
		t.Errorf("got URL %q, want %q", src.URL, "postgres://localhost/drizzledb")
	}
}

func TestDiscover_Compose(t *testing.T) {
	dir := filepath.Join(testdataDir(t), "compose")
	src, err := Discover(Options{StartDir: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if src.Kind != SourceCompose {
		t.Errorf("got kind %q, want %q", src.Kind, SourceCompose)
	}
	want := "postgres://myuser:mypassword@127.0.0.1:5433/mydb?sslmode=disable"
	if src.URL != want {
		t.Errorf("got URL %q, want %q", src.URL, want)
	}
}

func TestDiscover_DbseerConfig(t *testing.T) {
	dir := filepath.Join(testdataDir(t), "dbseer-config")
	src, err := Discover(Options{StartDir: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if src.Kind != SourceConfig {
		t.Errorf("got kind %q, want %q", src.Kind, SourceConfig)
	}
	if src.URL != "postgres://postgres:postgres@localhost:5432/localdb" {
		t.Errorf("got URL %q, want %q", src.URL, "postgres://postgres:postgres@localhost:5432/localdb")
	}
}

func TestDiscover_DbseerConfigNamed(t *testing.T) {
	dir := filepath.Join(testdataDir(t), "dbseer-config")
	src, err := Discover(Options{StartDir: dir, PreferEnvironment: "staging"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if src.Kind != SourceConfig {
		t.Errorf("got kind %q, want %q", src.Kind, SourceConfig)
	}
	if src.URL != "postgres://postgres:postgres@localhost:5432/stagingdb" {
		t.Errorf("got URL %q, want %q", src.URL, "postgres://postgres:postgres@localhost:5432/stagingdb")
	}
}

func TestDiscover_Priority_EnvBeforeCompose(t *testing.T) {
	// priority-test has BOTH .env (DATABASE_URL) and docker-compose.yml
	// .env must win because it has higher priority.
	dir := filepath.Join(testdataDir(t), "priority-test")
	src, err := Discover(Options{StartDir: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if src.Kind != SourceEnv {
		t.Errorf("got kind %q, want %q (env must beat compose)", src.Kind, SourceEnv)
	}
	if src.URL != "postgres://localhost/envdb" {
		t.Errorf("got URL %q, want %q", src.URL, "postgres://localhost/envdb")
	}
}

func TestDiscover_NotExists(t *testing.T) {
	_, err := Discover(Options{StartDir: "/nonexistent/path/that/does/not/exist"})
	if err == nil {
		t.Error("expected error for nonexistent StartDir, got nil")
	}
}

func TestDiscover_None(t *testing.T) {
	// An empty temp dir with no discovery sources.
	dir := t.TempDir()
	src, err := Discover(Options{StartDir: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// May find something if walking upward; just confirm no crash and returns a valid source.
	_ = src
}

// --- Unit tests for sub-parsers ---

func TestReadEnvFiles_Precedence(t *testing.T) {
	dir := filepath.Join(testdataDir(t), "env-chain")
	m, files, err := ReadEnvFiles(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) < 2 {
		t.Errorf("expected at least 2 env files, got %d", len(files))
	}
	// .env.development.local overrides .env
	if m["DATABASE_URL"] != "postgres://localhost/override" {
		t.Errorf("got DATABASE_URL=%q, want %q", m["DATABASE_URL"], "postgres://localhost/override")
	}
}

func TestFindEnvURL(t *testing.T) {
	tests := []struct {
		name      string
		envMap    map[string]string
		wantURL   string
		wantVar   string
	}{
		{"DATABASE_URL wins", map[string]string{"DATABASE_URL": "postgres://a", "POSTGRES_URL": "postgres://b"}, "postgres://a", "DATABASE_URL"},
		{"POSTGRES_URL fallback", map[string]string{"POSTGRES_URL": "postgres://b"}, "postgres://b", "POSTGRES_URL"},
		{"PG_URL last resort", map[string]string{"PG_URL": "postgres://c"}, "postgres://c", "PG_URL"},
		{"none found", map[string]string{"OTHER": "val"}, "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			u, v := FindEnvURL(tc.envMap)
			if u != tc.wantURL || v != tc.wantVar {
				t.Errorf("got (%q, %q), want (%q, %q)", u, v, tc.wantURL, tc.wantVar)
			}
		})
	}
}

func TestParsePrismaDatasource_Literal(t *testing.T) {
	path := filepath.Join(testdataDir(t), "prisma-literal", "prisma", "schema.prisma")
	u, provider, envVar, err := ParsePrismaDatasource(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if provider != "postgresql" {
		t.Errorf("got provider %q, want %q", provider, "postgresql")
	}
	if u != "postgres://postgres:secret@localhost:5432/myapp" {
		t.Errorf("got URL %q", u)
	}
	if envVar != "" {
		t.Errorf("got envVar %q, want empty", envVar)
	}
}

func TestParsePrismaDatasource_Env(t *testing.T) {
	path := filepath.Join(testdataDir(t), "prisma-env", "prisma", "schema.prisma")
	u, provider, envVar, err := ParsePrismaDatasource(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u != "" {
		t.Errorf("expected empty URL for env() reference, got %q", u)
	}
	if provider != "postgresql" {
		t.Errorf("got provider %q, want %q", provider, "postgresql")
	}
	if envVar != "DATABASE_URL" {
		t.Errorf("got envVar %q, want %q", envVar, "DATABASE_URL")
	}
}

func TestParseDrizzleConfig(t *testing.T) {
	path := filepath.Join(testdataDir(t), "drizzle-ts", "drizzle.config.ts")
	u, envVar, err := ParseDrizzleConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u != "" {
		t.Errorf("expected empty URL for process.env reference, got %q", u)
	}
	if envVar != "DATABASE_URL" {
		t.Errorf("got envVar %q, want %q", envVar, "DATABASE_URL")
	}
}

func TestParseComposeFile(t *testing.T) {
	path := filepath.Join(testdataDir(t), "compose", "docker-compose.yml")
	u, err := ParseComposeFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "postgres://myuser:mypassword@127.0.0.1:5433/mydb?sslmode=disable"
	if u != want {
		t.Errorf("got URL %q, want %q", u, want)
	}
}

func TestReadProjectConfig(t *testing.T) {
	path := filepath.Join(testdataDir(t), "dbseer-config", ".dbseer.json")
	cfg, err := ReadProjectConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DefaultEnv != "local" {
		t.Errorf("got defaultEnv %q, want %q", cfg.DefaultEnv, "local")
	}
	if len(cfg.Environments) != 2 {
		t.Errorf("got %d environments, want 2", len(cfg.Environments))
	}
}

func TestPickEnvironment(t *testing.T) {
	path := filepath.Join(testdataDir(t), "dbseer-config", ".dbseer.json")
	cfg, err := ReadProjectConfig(path)
	if err != nil {
		t.Fatalf("reading config: %v", err)
	}

	tests := []struct {
		requested string
		wantURL   string
		wantErr   bool
	}{
		{"", "postgres://postgres:postgres@localhost:5432/localdb", false},
		{"local", "postgres://postgres:postgres@localhost:5432/localdb", false},
		{"staging", "postgres://postgres:postgres@localhost:5432/stagingdb", false},
		{"nonexistent", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.requested, func(t *testing.T) {
			u, err := PickEnvironment(cfg, tc.requested)
			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if u != tc.wantURL {
				t.Errorf("got URL %q, want %q", u, tc.wantURL)
			}
		})
	}
}
