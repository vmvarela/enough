package git

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type ChangeKind string

const (
	Feature     ChangeKind = "feature"
	Fix         ChangeKind = "fix"
	Maintenance ChangeKind = "maintenance"
	Docs        ChangeKind = "docs"
	Refactor    ChangeKind = "refactor"
	Unknown     ChangeKind = "unknown"
)

type Commit struct {
	Hash string
	Date time.Time
	Kind ChangeKind
}
type History struct {
	Commits []Commit
	Latest  time.Time
	Oldest  time.Time
	Shallow bool
}
type Git interface {
	Inspect(context.Context, string, time.Time) (string, History, error)
}
type Local struct{}

func command(ctx context.Context, dir string, args ...string) ([]byte, error) {
	// Ignore inherited repository overrides without reading environment values.
	cmd := exec.CommandContext(ctx, "git", append([]string{"--no-optional-locks", "-c", "log.showSignature=false", "-c", "core.fsmonitor=false", "-C", dir}, args...)...)
	// Explicit arguments prevent repository selection through GIT_DIR/GIT_WORK_TREE.
	// A minimal environment also disables global config, pagers and hooks.
	cmd.Env = []string{"PATH=/usr/local/bin:/usr/bin:/bin", "HOME=/nonexistent", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "LC_ALL=C", "GIT_NO_LAZY_FETCH=1", "GIT_TERMINAL_PROMPT=0", "GIT_ALLOW_PROTOCOL="}
	return cmd.Output()
}

func (Local) Inspect(ctx context.Context, dir string, now time.Time) (string, History, error) {
	var h History
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", h, err
	}
	out, err := command(ctx, abs, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", h, fmt.Errorf("target must be a local Git working tree (and git must be installed)")
	}
	root := strings.TrimSpace(string(out))
	shallow, err := command(ctx, root, "rev-parse", "--is-shallow-repository")
	if err != nil {
		return "", h, fmt.Errorf("cannot inspect Git history")
	}
	h.Shallow = strings.TrimSpace(string(shallow)) == "true"
	// An unborn branch is valid, but a corrupt/unreadable history is an error.
	if _, err = command(ctx, root, "rev-parse", "--verify", "HEAD"); err != nil {
		refs, e := command(ctx, root, "for-each-ref", "--format=%(refname)")
		if e != nil || len(refs) > 0 {
			return "", h, fmt.Errorf("cannot read Git HEAD")
		}
		return root, h, nil
	}
	out, err = command(ctx, root, "log", "-1", "--format=%ct")
	if err != nil {
		return "", h, fmt.Errorf("cannot read latest commit")
	}
	h.Latest, err = parseTime(strings.TrimSpace(string(out)))
	if err != nil {
		return "", h, err
	}
	// Oldest reachable timestamp uses one bounded-output Git operation.
	out, err = command(ctx, root, "log", "--max-parents=0", "--max-count=1", "--format=%ct")
	if err != nil {
		return "", h, fmt.Errorf("cannot inspect repository age")
	}
	for _, s := range strings.Fields(string(out)) {
		d, e := parseTime(s)
		if e != nil {
			return "", h, e
		}
		if h.Oldest.IsZero() || d.Before(h.Oldest) {
			h.Oldest = d
		}
	}
	out, err = command(ctx, root, "log", "--max-count=100", "--since="+now.AddDate(-1, 0, 0).Format(time.RFC3339), "--format=%H%x00%ct%x00%s%x00")
	if err != nil {
		return "", h, fmt.Errorf("cannot read recent Git history")
	}
	fields := strings.Split(string(out), "\x00")
	for i := 0; i+2 < len(fields); i += 3 {
		d, e := parseTime(fields[i+1])
		if e != nil {
			return "", h, e
		}
		h.Commits = append(h.Commits, Commit{strings.TrimSpace(fields[i]), d, Classify(fields[i+2])})
	}
	return root, h, nil
}
func parseTime(s string) (time.Time, error) {
	n, e := strconv.ParseInt(s, 10, 64)
	if e != nil {
		return time.Time{}, fmt.Errorf("invalid Git timestamp")
	}
	return time.Unix(n, 0).UTC(), nil
}
func Classify(s string) ChangeKind {
	s = strings.ToLower(strings.TrimSpace(s))
	if i := strings.Index(s, ":"); i >= 0 {
		p := s[:i]
		if j := strings.Index(p, "("); j >= 0 {
			p = p[:j]
		}
		p = strings.TrimSuffix(p, "!")
		switch p {
		case "feat":
			return Feature
		case "fix":
			return Fix
		case "docs":
			return Docs
		case "chore", "build", "ci", "test":
			return Maintenance
		case "refactor", "perf":
			return Refactor
		}
	}
	words := strings.Fields(s)
	if len(words) == 0 {
		return Unknown
	}
	switch words[0] {
	case "add", "introduce", "implement":
		return Feature
	case "fix", "repair", "resolve":
		return Fix
	case "bump", "upgrade", "update":
		if strings.Contains(s, "depend") || strings.Contains(s, "compat") || strings.Contains(s, "version") {
			return Maintenance
		}
	case "document", "docs":
		return Docs
	case "refactor", "simplify":
		return Refactor
	}
	return Unknown
}
