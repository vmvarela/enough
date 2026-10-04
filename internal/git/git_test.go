package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestClassify(t *testing.T) {
	for _, tc := range []struct {
		s string
		k ChangeKind
	}{{"feat!: profiles", Feature}, {"feat(cli): --json", Feature}, {"fix: lock", Fix}, {"docs: install", Docs}, {"chore(deps): bump", Maintenance}, {"build: targets", Maintenance}, {"ci: actions", Maintenance}, {"test: coverage", Maintenance}, {"refactor: cleanup", Refactor}, {"perf: scan", Refactor}, {"Add JSON output", Feature}, {"Fix lock", Fix}, {"Bump dependencies", Maintenance}, {"Update dependencies", Maintenance}, {"Update wording", Unknown}, {"Document installation", Docs}, {"misc work", Unknown}} {
		if got := Classify(tc.s); got != tc.k {
			t.Errorf("%q: %s != %s", tc.s, got, tc.k)
		}
	}
}
func run(t *testing.T, dir string, env []string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if env != nil {
		cmd.Env = append(os.Environ(), env...)
	}
	b, e := cmd.CombinedOutput()
	if e != nil {
		t.Fatalf("git %v: %v: %s", args, e, b)
	}
	return string(b)
}
func TestLocalGitHistory(t *testing.T) {
	root := t.TempDir()
	run(t, root, nil, "init", "-b", "main")
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	got, h, e := (Local{}).Inspect(context.Background(), root, now)
	if e != nil || got != root || len(h.Commits) != 0 {
		t.Fatalf("unborn: %s %+v %v", got, h, e)
	}
	identity := []string{"GIT_AUTHOR_NAME=Fixture", "GIT_COMMITTER_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_COMMITTER_EMAIL=fixture@example.invalid"}
	old := append(identity, "GIT_AUTHOR_DATE=2024-01-01T12:00:00Z", "GIT_COMMITTER_DATE=2024-01-01T12:00:00Z")
	run(t, root, old, "commit", "--allow-empty", "-m", "feat: original")
	recent := append(identity, "GIT_AUTHOR_DATE=2026-10-01T12:00:00Z", "GIT_COMMITTER_DATE=2026-10-01T12:00:00Z")
	run(t, root, recent, "commit", "--allow-empty", "-m", "fix: secret-token-value\n\nbody")
	sub := filepath.Join(root, "sub")
	os.Mkdir(sub, 0755)
	// Git-related environment values must not select a different repository.
	t.Setenv("GIT_DIR", "/does/not/exist")
	t.Setenv("GIT_WORK_TREE", "/does/not/exist")
	got, h, e = (Local{}).Inspect(context.Background(), sub, now)
	if e != nil || got != root || len(h.Commits) != 1 || h.Commits[0].Kind != Fix || h.Oldest.Year() != 2024 {
		t.Fatalf("history: %s %+v %v", got, h, e)
	}
	// Commit bodies/messages are never retained in analysis data.
	if strings.Contains(h.Commits[0].Hash, "secret-token") {
		t.Fatal("leaked message")
	}
}
func TestNonRepositoryAndCancellation(t *testing.T) {
	if _, _, e := (Local{}).Inspect(context.Background(), t.TempDir(), time.Now()); e == nil {
		t.Fatal("accepted non-repository")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, e := (Local{}).Inspect(ctx, t.TempDir(), time.Now()); e == nil {
		t.Fatal("ignored cancellation")
	}
}

func TestGitCannotUseNetworkTransports(t *testing.T) {
	root := t.TempDir()
	run(t, root, nil, "init", "-q")
	// Even an explicit repository setting cannot override the empty allowlist.
	run(t, root, nil, "config", "protocol.https.allow", "always")
	_, e := command(context.Background(), root, "ls-remote", "https://example.invalid/repository.git")
	exit, ok := e.(*exec.ExitError)
	if !ok || !strings.Contains(string(exit.Stderr), "transport 'https' not allowed") {
		t.Fatalf("Git did not deny transport locally: %v", e)
	}
}
