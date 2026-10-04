package cli

import (
	"bytes"
	"context"
	"errors"
	"github.com/vmvarela/enough/internal/analysis"
	"github.com/vmvarela/enough/internal/project"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fake struct {
	r    *analysis.Result
	err  error
	path string
}

func (f *fake) Analyze(_ context.Context, path string) (*analysis.Result, error) {
	f.path = path
	return f.r, f.err
}
func TestCommandsAndExitCodes(t *testing.T) {
	for _, tc := range []struct {
		state analysis.State
		code  int
	}{{analysis.Enough, 0}, {analysis.Resting, 0}, {analysis.Unfinished, 1}, {analysis.Growing, 2}, {analysis.Overgrown, 3}, {analysis.Unknown, 4}} {
		for _, args := range [][]string{nil, {"check"}, {"why"}, {"next"}, {"--json"}, {"check", "--json"}, {"--json", "why"}} {
			f := &fake{r: &analysis.Result{SchemaVersion: 1, State: tc.state, Recommendations: []string{}}}
			_ = f
			var out, err bytes.Buffer
			r := &analysis.Result{SchemaVersion: 1, State: tc.state, Recommendations: []string{}}
			f.r = r
			if code := Run(context.Background(), args, &out, &err, "0.1.0", f, time.Now); code != tc.code {
				t.Fatalf("%v %s: %d %s", args, tc.state, code, err.String())
			}
		}
	}
}
func TestArgumentErrorsAndRemember(t *testing.T) {
	f := &fake{r: &analysis.Result{SchemaVersion: 1, Root: t.TempDir(), State: analysis.Enough, Purpose: project.Purpose{Statement: "Print certificate expiration."}}}
	for _, args := range [][]string{{"check", "--force"}, {"remember", "--json"}, {"version", "--json"}, {"--bad"}, {"check", "one", "two"}} {
		var o, e bytes.Buffer
		if c := Run(context.Background(), args, &o, &e, "0.1.0", f, time.Now); c != 10 {
			t.Fatalf("accepted %v", args)
		}
	}
	var o, e bytes.Buffer
	if c := Run(context.Background(), []string{"version"}, &o, &e, "0.1.0", f, time.Now); c != 0 || o.String() != "enough 0.1.0\n" {
		t.Fatal(o.String())
	}
	o.Reset()
	if c := Run(context.Background(), []string{"remember"}, &o, &e, "0.1.0", f, time.Now); c != 0 {
		t.Fatal(e.String())
	}
	if _, err := os.Stat(filepath.Join(f.r.Root, "ENOUGH.md")); err != nil {
		t.Fatal(err)
	}
	if c := Run(context.Background(), []string{"remember"}, &o, &e, "0.1.0", f, time.Now); c != 10 {
		t.Fatal("overwrote declaration")
	}
	if c := Run(context.Background(), []string{"remember", "--force"}, &o, &e, "0.1.0", f, time.Now); c != 0 {
		t.Fatal(e.String())
	}
	f.err = errors.New("configuration error")
	if c := Run(context.Background(), nil, &o, &e, "0.1.0", f, time.Now); c != 10 {
		t.Fatal("wrong error exit")
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("closed") }
func TestOutputFailureAndLiteralPath(t *testing.T) {
	f := &fake{r: &analysis.Result{State: analysis.Enough}}
	var err bytes.Buffer
	if c := Run(context.Background(), nil, failingWriter{}, &err, "0.1.0", f, time.Now); c != 10 {
		t.Fatal("ignored output error")
	}
	var out bytes.Buffer
	Run(context.Background(), []string{"check", "--", "-repo"}, &out, &err, "0.1.0", f, time.Now)
	if f.path != "-repo" {
		t.Fatal(f.path)
	}
	if !strings.Contains(err.String(), "cannot write output") {
		t.Fatal(err.String())
	}
}
