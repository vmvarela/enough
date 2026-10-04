package project

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func put(t *testing.T, root, path, text string) {
	t.Helper()
	p := filepath.Join(root, path)
	if e := os.MkdirAll(filepath.Dir(p), 0755); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(p, []byte(text), 0644); e != nil {
		t.Fatal(e)
	}
}
func inspect(t *testing.T, root string) Snapshot {
	t.Helper()
	s, e := Inspect(context.Background(), root)
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func TestPurposePrecedence(t *testing.T) {
	r := t.TempDir()
	put(t, r, "package.json", `{"name":"cert-days","description":"Inspect certificate expiration locally."}`)
	if s := inspect(t, r); s.Purpose.Source != "package.json" {
		t.Fatal(s.Purpose)
	}
	put(t, r, "README.md", "# cert-days\n\nPrint TLS certificate expiration from the command line.\n")
	if s := inspect(t, r); s.Purpose.Source != "inferred from README.md" {
		t.Fatal(s.Purpose)
	}
	put(t, r, "enough.toml", "version = 1\n[purpose]\nstatement = 'Check certificate expiration.'\n")
	if s := inspect(t, r); s.Purpose.Source != "enough.toml" {
		t.Fatal(s.Purpose)
	}
}
func TestTodoAndPromises(t *testing.T) {
	r := t.TempDir()
	put(t, r, "README.md", "# cert-days\n\nPrint certificate expiration from a small CLI.\n\n## Features\n- JSON output\n")
	put(t, r, "main.go", "package main\n// TODO: tidy variable names\nfunc main() {}\n")
	s := inspect(t, r)
	if s.Todos != 1 || s.Promises[0].Status != "unclear" {
		t.Fatal(s.Promises)
	}
	put(t, r, "main.go", "package main\n// FIXME: implement JSON output\nfunc main() {}\n")
	s = inspect(t, r)
	if s.Promises[0].Status != "missing" {
		t.Fatal(s.Promises)
	}
	put(t, r, "main.go", "package main\nfunc output() string { return \"json\" }\n")
	put(t, r, "main_test.go", "package main\nfunc TestJSON() { want := \"json\"; _ = want }\n")
	s = inspect(t, r)
	if s.Promises[0].Status != "satisfied" {
		t.Fatal(s.Promises)
	}
}
func TestScopeNeedsAffirmativeDocumentation(t *testing.T) {
	r := t.TempDir()
	put(t, r, "enough.toml", "version = 1\n[purpose]\nstatement = 'Print certificate expiration.'\n[scope]\nexcludes = ['dashboards']\n")
	put(t, r, "dashboard/main.go", "package dashboard\nfunc Run() {}\n")
	for _, docs := range []string{"# Features\n- No dashboards\n", "# Exclusions\n- dashboards\n", "# Features\n- dashboards are not included\n", "# Features\n```\n- dashboards\n```"} {
		put(t, r, "README.md", docs)
		if s := inspect(t, r); len(s.Drift) > 0 {
			t.Fatal("exclusion mistaken for growth", docs)
		}
	}
	put(t, r, "README.md", "# Features\n- dashboards\n")
	if s := inspect(t, r); len(s.Drift) != 1 {
		t.Fatal("missed advertised excluded capability")
	}
	os.Remove(filepath.Join(r, "dashboard/main.go"))
	if s := inspect(t, r); len(s.Drift) > 0 {
		t.Fatal("documentation alone is insufficient")
	}
}
func TestLimitsAndPrivacy(t *testing.T) {
	r := t.TempDir()
	put(t, r, "README.md", "# Tool\n\nPrint certificate expiration with this small tool.\n")
	put(t, r, "vendor/hidden.go", "// TODO: secret-token\n")
	put(t, r, "testdata/hidden.go", "// TODO: secret-token\n")
	put(t, r, ".env", "SECRET=secret-token")
	put(t, r, "binary.go", "\x00TODO secret-token")
	put(t, r, "large.go", strings.Repeat("x", MaxFileSize+1))
	outside := t.TempDir()
	put(t, outside, "secret.go", "// TODO: secret-token\n")
	if e := os.Symlink(filepath.Join(outside, "secret.go"), filepath.Join(r, "escape.go")); e != nil {
		t.Fatal(e)
	}
	s := inspect(t, r)
	if !s.Truncated || s.Todos != 0 || len(s.Files) != 0 {
		t.Fatalf("unsafe snapshot: %+v", s)
	}
	if e := os.Symlink(filepath.Join(outside, "secret.go"), filepath.Join(r, "enough.toml")); e != nil {
		t.Fatal(e)
	}
	if _, e := Inspect(context.Background(), r); e == nil {
		t.Fatal("accepted symlink configuration")
	}
}
func TestCanceledInspection(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := Inspect(ctx, t.TempDir()); e == nil {
		t.Fatal("ignored cancellation")
	}
}

func TestScopeDoesNotMatchSubstrings(t *testing.T) {
	root := t.TempDir()
	put(t, root, "enough.toml", "version = 1\n[purpose]\nstatement = 'Print certificate expiration.'\n[scope]\nexcludes = ['web UI', 'plugins']\n")
	put(t, root, "README.md", "# Features\n- websocket connections\n- unplugging connections\n")
	put(t, root, "websocket/main.go", "package websocket\nfunc Connect() {}\n")
	put(t, root, "unplugging/main.go", "package unplugging\nfunc Run() {}\n")
	if s := inspect(t, root); len(s.Drift) > 0 {
		t.Fatal("substring is not corroborated scope drift")
	}
}

func TestVisitLimit(t *testing.T) {
	root := t.TempDir()
	for i := 0; i <= MaxFiles; i++ {
		put(t, root, fmt.Sprintf("file-%05d.txt", i), "")
	}
	if s := inspect(t, root); !s.Truncated {
		t.Fatal("did not enforce file limit")
	}
}

func BenchmarkInspection(b *testing.B) {
	root := b.TempDir()
	os.WriteFile(filepath.Join(root, "README.md"), []byte("# certificates\n\nPrint certificate expiration from this small CLI.\n\n## Features\n- certificate expiration\n- JSON output\n"), 0644)
	source := strings.Repeat("func certificate() string { return \"certificate expiration json\" }\n", 500)
	for i := 0; i < 30; i++ {
		os.WriteFile(filepath.Join(root, fmt.Sprintf("file_%d.go", i)), []byte(source), 0644)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, e := Inspect(context.Background(), root); e != nil {
			b.Fatal(e)
		}
	}
}
