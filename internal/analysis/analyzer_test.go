package analysis_test

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/vmvarela/enough/internal/analysis"
	"github.com/vmvarela/enough/internal/evidence"
	gitrepo "github.com/vmvarela/enough/internal/git"
	"github.com/vmvarela/enough/internal/project"
	"github.com/vmvarela/enough/internal/report"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var now = time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)

func history(state string) gitrepo.History {
	h := gitrepo.History{Oldest: now.AddDate(0, -6, 0), Latest: now.AddDate(0, 0, -1)}
	kinds := []gitrepo.ChangeKind{gitrepo.Fix, gitrepo.Docs}
	if state == "growing" {
		kinds = []gitrepo.ChangeKind{gitrepo.Feature, gitrepo.Feature, gitrepo.Feature, gitrepo.Feature, gitrepo.Fix}
	}
	if state == "resting" {
		h.Oldest = now.AddDate(-2, 0, 0)
	}
	for _, k := range kinds {
		h.Commits = append(h.Commits, gitrepo.Commit{Date: h.Latest, Kind: k})
	}
	return h
}
func TestSixStateFixtures(t *testing.T) {
	for _, state := range []string{"unfinished", "growing", "enough", "resting", "overgrown", "unknown"} {
		t.Run(state, func(t *testing.T) {
			root := filepath.Join("..", "..", "testdata", state)
			s, e := project.Inspect(context.Background(), root)
			if e != nil {
				t.Fatal(e)
			}
			r := analysis.Decide(s, history(state), now)
			if string(r.State) != state {
				b, _ := json.MarshalIndent(r, "", "  ")
				t.Fatalf("want %s, got %s\n%s", state, r.State, b)
			}
			if len(r.Recommendations) > 3 {
				t.Fatal("enough is suggesting too much")
			}
			for _, cmd := range []string{"check", "why", "next"} {
				var b bytes.Buffer
				if e = report.Text(&b, r, cmd); e != nil {
					t.Fatal(e)
				}
				path := filepath.Join(root, cmd+".golden")
				if os.Getenv("UPDATE_GOLDEN") == "1" {
					if e = os.WriteFile(path, b.Bytes(), 0644); e != nil {
						t.Fatal(e)
					}
				}
				want, e := os.ReadFile(path)
				if e != nil {
					t.Fatal(e)
				}
				if b.String() != string(want) {
					t.Fatalf("%s output changed\n%s", cmd, b.String())
				}
			}
			var b bytes.Buffer
			if e = report.JSON(&b, r); e != nil {
				t.Fatal(e)
			}
			jsonPath := filepath.Join(root, "json.golden")
			if os.Getenv("UPDATE_GOLDEN") == "1" {
				if e = os.WriteFile(jsonPath, b.Bytes(), 0644); e != nil {
					t.Fatal(e)
				}
			}
			wantJSON, e := os.ReadFile(jsonPath)
			if e != nil {
				t.Fatal(e)
			}
			if !bytes.Equal(wantJSON, b.Bytes()) {
				t.Fatal("JSON schema/output changed")
			}
			var got analysis.Result
			if e = json.Unmarshal(b.Bytes(), &got); e != nil {
				t.Fatal(e)
			}
			if got.SchemaVersion != 1 || got.State != r.State || got.Recommendations == nil {
				t.Fatal("unstable JSON contract")
			}
			if strings.Count(mustText(r), "\n") > 20 {
				t.Fatal("default output exceeds one screen")
			}
		})
	}
}
func mustText(r *analysis.Result) string {
	var b bytes.Buffer
	report.Text(&b, r, "check")
	return b.String()
}
func TestDecisionPrecedenceAndConservatism(t *testing.T) {
	s, e := project.Inspect(context.Background(), "../../testdata/enough")
	if e != nil {
		t.Fatal(e)
	}
	drift := evidence.Evidence{Kind: "scope", Confidence: evidence.High}
	cases := []struct {
		name string
		edit func(*project.Snapshot, *gitrepo.History)
		want analysis.State
	}{
		{"missing beats drift and growth", func(s *project.Snapshot, h *gitrepo.History) {
			s.Promises[0].Status = "missing"
			s.Drift = []evidence.Evidence{drift}
		}, analysis.Unfinished},
		{"drift beats growth", func(s *project.Snapshot, h *gitrepo.History) { s.Drift = []evidence.Evidence{drift} }, analysis.Overgrown},
		{"unclear is not enough", func(s *project.Snapshot, h *gitrepo.History) {
			s.Promises[0].Status = "unclear"
			*h = history("enough")
		}, analysis.Unknown},
		{"truncated is not enough", func(s *project.Snapshot, h *gitrepo.History) { s.Truncated = true; *h = history("enough") }, analysis.Unknown},
		{"shallow is not resting", func(s *project.Snapshot, h *gitrepo.History) { *h = history("resting"); h.Shallow = true }, analysis.Enough},
		{"inactivity is not completion", func(s *project.Snapshot, h *gitrepo.History) {
			s.HasImplementation = false
			*h = history("resting")
			h.Commits = nil
		}, analysis.Unknown},
		{"unknown commits are not stability", func(s *project.Snapshot, h *gitrepo.History) {
			*h = history("resting")
			h.Commits = append(h.Commits, gitrepo.Commit{Kind: gitrepo.Unknown})
		}, analysis.Enough},
		{"old features are not active growth", func(s *project.Snapshot, h *gitrepo.History) { h.Latest = now.AddDate(0, -6, 0) }, analysis.Enough},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			copy := s
			copy.Promises = append([]project.Promise{}, s.Promises...)
			h := history("growing")
			c.edit(&copy, &h)
			if r := analysis.Decide(copy, h, now); r.State != c.want {
				t.Fatalf("want %s, got %s", c.want, r.State)
			}
		})
	}
	s.Promises = nil
	for i := 0; i < 100; i++ {
		s.Promises = append(s.Promises, project.Promise{Text: "certificate expiration", Status: "missing"})
	}
	if len(analysis.Decide(s, history("enough"), now).Recommendations) != 3 {
		t.Fatal("recommendation limit")
	}
}
