package remember

import (
	"github.com/vmvarela/enough/internal/analysis"
	"github.com/vmvarela/enough/internal/project"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRememberProtectsExistingFiles(t *testing.T) {
	r := &analysis.Result{State: analysis.Enough, Root: t.TempDir(), Purpose: project.Purpose{Statement: "Print certificate expiration."}, Includes: []string{"certificates"}, Excludes: []string{"dashboards"}}
	now := time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)
	if e := Write(r, now, false); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(r.Root, "ENOUGH.md")
	b, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(string(b), "Declared enough on 2026-10-04.") || !strings.Contains(string(b), "- dashboards") {
		t.Fatal(string(b))
	}
	if e = Write(r, now, false); e == nil {
		t.Fatal("overwrote existing declaration")
	}
	r.Purpose.Statement = "A changed boundary."
	if e = Write(r, now, true); e != nil {
		t.Fatal(e)
	}
	b, _ = os.ReadFile(path)
	if !strings.Contains(string(b), "A changed boundary.") {
		t.Fatal("force did not replace")
	}
	outside := filepath.Join(t.TempDir(), "target")
	os.WriteFile(outside, []byte("untouched"), 0644)
	os.Remove(path)
	os.Symlink(outside, path)
	if e = Write(r, now, true); e == nil {
		t.Fatal("followed symlink")
	}
	b, _ = os.ReadFile(outside)
	if string(b) != "untouched" {
		t.Fatal("modified external target")
	}
}
func TestUnverifiedDeclarationIsHonest(t *testing.T) {
	r := &analysis.Result{State: analysis.Unknown, Purpose: project.Purpose{Statement: "unclear"}}
	text := Render(r, time.Now())
	if !strings.Contains(text, "has not\nverified") || strings.Contains(text, "appear implemented") {
		t.Fatal(text)
	}
}
