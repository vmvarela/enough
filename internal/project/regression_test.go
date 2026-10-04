package project

import (
	"strings"
	"testing"
)

func TestPurposeParagraphsAndAsides(t *testing.T) {
	readme := "# Ghoten\n\n> **Name origin:** Ghoten is a Dragon Ball reference.\n\n[![CI](badge)](ci)\n\n## What is this?\n\nGhoten is an [OpenTofu](https://opentofu.org/) fork\nthat stores state in OCI registries.\n\n## Usage\n\nRun the application.\n"
	want := "Ghoten is an OpenTofu fork that stores state in OCI registries."
	if got := inferPurpose(readme); got != want {
		t.Fatalf("%q != %q", got, want)
	}
	if got := inferPurpose("# Tool\n\n## Installation\n\nInstall this tool using the following commands.\n"); got != "" {
		t.Fatalf("installation is not purpose: %q", got)
	}
}
func TestPromiseReferencesShareSpecificTerms(t *testing.T) {
	files := []TextFile{indexFile("main.go", "func certificateExpiration() {}", false), indexFile("main_test.go", "func TestCertificateExpiration(t *testing.T) {}", true)}
	if a, b := supportingPair(files, tokens("certificate expiration with configurable formatting")); a != "" || b != "" {
		t.Fatal("unsupported formatting was guessed")
	}
	if a, b := supportingPair(files, tokens("certificate expiration output")); a == "" || b == "" {
		t.Fatal("identifier references were not recognized")
	}
	files = []TextFile{indexFile("main.go", "json streamed", false), indexFile("main_test.go", "json rows", true)}
	if a, b := supportingPair(files, tokens("JSON output for streamed rows")); a != "" || b != "" {
		t.Fatal("different terms cannot corroborate each other")
	}
	files = []TextFile{indexFile("main.go", "json rows", false), indexFile("main_test.go", "json rows", true)}
	if a, b := supportingPair(files, tokens("JSON output for streamed rows")); a == "" || b == "" {
		t.Fatal("corroborated lexical evidence was missed")
	}
}
func TestZigInlineTestsAreSeparateEvidence(t *testing.T) {
	root := t.TempDir()
	put(t, root, "README.md", "# Certificate\n\nPrint certificate expiration from a small CLI.\n\n## Features\n- certificate expiration\n")
	src := `pub fn certificateExpiration() void {}
// test "fake" {}
const example = "test fake { }";
test "certificate expiration" {
 const braces = "}}}";
 if (true) { _ = braces; }
 // TODO: certificate expiration
}
test { const text = "anonymous"; _ = text; }
`
	put(t, root, "main.zig", src)
	s := inspect(t, root)
	if s.Tests != 1 || s.InlineTests != 2 || s.Promises[0].Status != "satisfied" || s.Todos != 0 {
		t.Fatalf("%+v", s)
	}
	// A capability appearing only in a test is not an implementation.
	put(t, root, "main.zig", strings.Replace(src, "pub fn certificateExpiration() void {}", "pub fn unrelated() void {}", 1))
	s = inspect(t, root)
	if s.Promises[0].Status != "unclear" {
		t.Fatal("test content leaked into implementation evidence", s.Promises)
	}
}
func TestZigFakeAndUnterminatedTests(t *testing.T) {
	for _, src := range []string{"// test \"fake\" {}", `const text = "test fake {}";`, "\\\\test \"fake\" {}\n", "test \"broken\" {"} {
		if got := zigTests(src); len(got) != 0 {
			t.Fatalf("fake test: %q: %+v", src, got)
		}
	}
}
func TestUsageSubsectionsAndShellComments(t *testing.T) {
	s := Snapshot{ReadmePath: "README.md", Readme: "# CLI\n\n## Usage\n\n### JSON\n\n```sh\n# Print JSON\ncli --json\n```\n\n## Installation\n```sh\ninstaller --unsupported\n```", Files: []TextFile{indexFile("main.go", `func run() { print("json") }`, false), indexFile("main_test.go", `func TestJSON() { print("json") }`, true)}}
	ps := Promises(s)
	if len(ps) != 1 || ps[0].Text != "--json" || ps[0].Status != "satisfied" {
		t.Fatalf("%+v", ps)
	}
}
