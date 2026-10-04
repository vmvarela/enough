package analysis

import (
	"context"
	"fmt"
	"github.com/vmvarela/enough/internal/evidence"
	gitrepo "github.com/vmvarela/enough/internal/git"
	"github.com/vmvarela/enough/internal/project"
	"time"
)

type State string

const (
	Unfinished State = "unfinished"
	Growing    State = "growing"
	Enough     State = "enough"
	Resting    State = "resting"
	Overgrown  State = "overgrown"
	Unknown    State = "unknown"
)

type Changes struct {
	Feature     int `json:"feature"`
	Fix         int `json:"fix"`
	Maintenance int `json:"maintenance"`
	Docs        int `json:"docs"`
	Refactor    int `json:"refactor"`
	Unknown     int `json:"unknown"`
}
type Analysis struct {
	Promises    []project.Promise `json:"promises"`
	Changes     Changes           `json:"changes"`
	Stability   string            `json:"stability"`
	Growth      string            `json:"growth"`
	Scope       string            `json:"scope"`
	Maintenance string            `json:"maintenance"`
	Tests       int               `json:"tests_detected"`
	Todos       int               `json:"todo_markers"`
	Truncated   bool              `json:"truncated"`
}
type Result struct {
	SchemaVersion   int                 `json:"schema_version"`
	Name            string              `json:"name"`
	State           State               `json:"state"`
	Summary         string              `json:"summary"`
	Purpose         project.Purpose     `json:"purpose"`
	Recommendations []string            `json:"recommendations"`
	Evidence        []evidence.Evidence `json:"evidence"`
	Analysis        Analysis            `json:"analysis"`
	// Declaration fields are intentionally not a second public JSON schema.
	Includes []string `json:"-"`
	Excludes []string `json:"-"`
	Root     string   `json:"-"`
}
type Analyzer interface {
	Analyze(context.Context, string) (*Result, error)
}
type Local struct {
	Git gitrepo.Git
	Now func() time.Time
}

func (a Local) Analyze(ctx context.Context, repo string) (*Result, error) {
	now := time.Now().UTC()
	if a.Now != nil {
		now = a.Now()
	}
	g := a.Git
	if g == nil {
		g = gitrepo.Local{}
	}
	root, h, e := g.Inspect(ctx, repo, now)
	if e != nil {
		return nil, e
	}
	s, e := project.Inspect(ctx, root)
	if e != nil {
		return nil, e
	}
	r := Decide(s, h, now)
	r.Root = root
	return r, nil
}
func Decide(s project.Snapshot, h gitrepo.History, now time.Time) *Result {
	r := &Result{SchemaVersion: 1, Name: s.Name, Purpose: s.Purpose, Recommendations: []string{}, Evidence: append([]evidence.Evidence{}, s.Evidence...), Includes: s.Config.Scope.Includes, Excludes: s.Config.Scope.Excludes}
	a := Analysis{Promises: s.Promises, Tests: s.Tests, Todos: s.Todos, Truncated: s.Truncated, Stability: "unclear", Growth: "unclear", Scope: "no strong drift found", Maintenance: "unclear"}
	missing, satisfied := 0, 0
	for _, p := range s.Promises {
		switch p.Status {
		case "missing":
			missing++
			r.Evidence = append(r.Evidence, evidence.Evidence{Kind: "promise", Message: "A declared promise has a related unfinished marker: " + p.Text + ".", Source: p.Source + " + " + join(p.Supporting), Confidence: evidence.High})
			if len(r.Recommendations) < 3 {
				r.Recommendations = append(r.Recommendations, "Resolve the unfinished promise: "+p.Text+".")
			}
		case "satisfied":
			satisfied++
			r.Evidence = append(r.Evidence, evidence.Evidence{Kind: "promise", Message: "Source and test references support: " + p.Text + " (tests were not run).", Source: p.Source + " + " + join(p.Supporting), Confidence: evidence.Medium})
		default:
			r.Evidence = append(r.Evidence, evidence.Evidence{Kind: "promise", Message: "Unable to verify: " + p.Text + ".", Source: p.Source, Confidence: evidence.Low})
		}
	}
	for _, c := range h.Commits {
		switch c.Kind {
		case gitrepo.Feature:
			a.Changes.Feature++
		case gitrepo.Fix:
			a.Changes.Fix++
		case gitrepo.Maintenance:
			a.Changes.Maintenance++
		case gitrepo.Docs:
			a.Changes.Docs++
		case gitrepo.Refactor:
			a.Changes.Refactor++
		default:
			a.Changes.Unknown++
		}
	}
	meaningful := a.Changes.Feature + a.Changes.Fix + a.Changes.Maintenance + a.Changes.Refactor
	total := len(h.Commits)
	if total > 0 {
		r.Evidence = append(r.Evidence, evidence.Evidence{Kind: "history", Message: fmt.Sprintf("%d of %d inspected commits introduce features; %d are fixes or maintenance.", a.Changes.Feature, total, a.Changes.Fix+a.Changes.Maintenance), Source: "git log (up to 100 commits within 12 months)", Confidence: evidence.Medium})
		a.Maintenance = "active"
	} else if !h.Latest.IsZero() {
		a.Maintenance = "quiet"
		r.Evidence = append(r.Evidence, evidence.Evidence{Kind: "history", Message: "No commits fall within the analysis window.", Source: "git log", Confidence: evidence.Medium})
	}
	expanding := a.Changes.Feature >= 3 && a.Changes.Feature*2 > meaningful && !h.Latest.IsZero() && now.Sub(h.Latest) <= 90*24*time.Hour
	if expanding {
		a.Growth = "expanding"
	} else if meaningful > 0 {
		a.Growth = "no dominant feature growth"
	}
	// Message classification supports a stability pattern, not a verified API claim.
	stable := !h.Shallow && !h.Oldest.IsZero() && now.Sub(h.Oldest) >= 365*24*time.Hour && a.Changes.Feature == 0 && a.Changes.Unknown == 0 && a.Changes.Refactor == 0
	complete := s.Purpose.Confidence != evidence.Low && s.HasImplementation && len(s.Promises) > 0 && satisfied == len(s.Promises) && !s.Truncated
	if stable {
		a.Stability = "maintenance pattern"
	}
	r.Evidence = append(r.Evidence, s.Drift...)
	if len(s.Drift) > 0 {
		a.Scope = "possible drift"
	}
	if h.Shallow {
		r.Evidence = append(r.Evidence, evidence.Evidence{Kind: "history", Message: "History is shallow; long-term stability cannot be established.", Source: "git rev-parse", Confidence: evidence.High})
	}
	switch {
	case missing > 0:
		r.State = Unfinished
		r.Summary = "An advertised promise has explicit evidence of unfinished work."
	case len(s.Drift) > 0:
		r.State = Overgrown
		r.Summary = "The project may have grown beyond its declared purpose."
		r.Recommendations = []string{"Decide whether the expanded scope still belongs to this project."}
	case expanding:
		r.State = Growing
		r.Summary = "This project appears to still be changing shape."
	case complete && stable:
		r.State = Resting
		r.Summary = "The project appears complete; its history suggests maintenance or rest."
	case complete:
		r.State = Enough
		r.Summary = "The project's stated promises appear supported by implementation and tests."
	default:
		r.State = Unknown
		r.Summary = "There is not enough evidence for a meaningful conclusion."
	}
	r.Analysis = a
	return r
}
func join(xs []string) string {
	out := ""
	for i, x := range xs {
		if i > 0 {
			out += " + "
		}
		out += x
	}
	return out
}
func ExitCode(s State) int {
	switch s {
	case Enough, Resting:
		return 0
	case Unfinished:
		return 1
	case Growing:
		return 2
	case Overgrown:
		return 3
	default:
		return 4
	}
}
