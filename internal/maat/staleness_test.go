package maat

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// --------------------------------------------------------------------------- #
// git staleness helpers (ADR 0012)
// --------------------------------------------------------------------------- #

const staleDoc = "docs/architecture/modules/m.md"

// git runs a git command in dir with a fixed identity and no signing, failing
// the test on error.
func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{"-C", dir,
		"-c", "user.name=Test", "-c", "user.email=test@example.com",
		"-c", "commit.gpgsign=false", "-c", "init.defaultBranch=main"}, args...)
	out, err := exec.Command("git", full...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func commitAll(t *testing.T, root, msg string) {
	t.Helper()
	git(t, root, "add", "-A")
	git(t, root, "commit", "-q", "-m", msg)
}

// gitRepoWithDoc scaffolds a Ma'at repo inside a fresh git repository with one
// doc covering src/thing.py, all committed together in a single commit — so
// the starting state is "docs in sync".
func gitRepoWithDoc(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	root := initRepo(t)
	git(t, root, "init", "-q")
	writeFile(t, root, "src/thing.py", "print('hi')\n")
	writeFile(t, root, staleDoc,
		"---\ntitle: M\nstatus: current\nsummary: x\nrelated_code:\n  - src/thing.py\n---\n# M\n")
	run(t, "sync", root)
	commitAll(t, root, "initial")
	return root
}

func checkOut(t *testing.T, root string) (string, int) {
	t.Helper()
	out, _, code := run(t, "check", root)
	return out, code
}

func hasStaleFinding(out, where string) bool {
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "[staleness]") && strings.Contains(line, where) {
			return true
		}
	}
	return false
}

// --------------------------------------------------------------------------- #
// tests
// --------------------------------------------------------------------------- #

func TestGitStalenessCleanWhenCommittedTogether(t *testing.T) {
	root := gitRepoWithDoc(t)
	out, code := checkOut(t, root)
	if code != 0 || hasStaleFinding(out, staleDoc) {
		t.Fatalf("code and doc committed together must not be stale (exit %d):\n%s", code, out)
	}
}

func TestGitStalenessFlagsLaterCodeCommit(t *testing.T) {
	root := gitRepoWithDoc(t)
	writeFile(t, root, "src/thing.py", "print('changed')\n")
	commitAll(t, root, "change code only")

	out, code := checkOut(t, root)
	if code != 0 {
		t.Fatalf("staleness is a warning by default, got exit %d:\n%s", code, out)
	}
	if !hasStaleFinding(out, staleDoc) {
		t.Fatalf("expected staleness for %s, got:\n%s", staleDoc, out)
	}
	if strings.Contains(out, "mtime") {
		t.Errorf("git mode should not mention mtimes:\n%s", out)
	}

	// Updating the doc in a later commit clears it.
	writeFile(t, root, staleDoc,
		"---\ntitle: M\nstatus: current\nsummary: y\nrelated_code:\n  - src/thing.py\n---\n# M\n\nUpdated.\n")
	run(t, "sync", root)
	commitAll(t, root, "update doc")
	out, _ = checkOut(t, root)
	if hasStaleFinding(out, staleDoc) {
		t.Errorf("doc updated after code must clear staleness, got:\n%s", out)
	}
}

func TestGitStalenessIgnoresMtimeOnlyChanges(t *testing.T) {
	root := gitRepoWithDoc(t)
	// The ADR 0004 false positive: a formatter or checkout bumps the mtime
	// without changing content. Git sees no change, so nothing is stale.
	makeNewer(t, filepath.Join(root, "src/thing.py"))
	out, _ := checkOut(t, root)
	if hasStaleFinding(out, staleDoc) {
		t.Errorf("an mtime-only change must not be stale in git mode, got:\n%s", out)
	}
}

func TestGitStalenessUncommittedChanges(t *testing.T) {
	root := gitRepoWithDoc(t)
	writeFile(t, root, "src/thing.py", "print('wip')\n")
	out, _ := checkOut(t, root)
	if !hasStaleFinding(out, staleDoc) {
		t.Fatalf("dirty code with a clean doc must be stale, got:\n%s", out)
	}

	// Editing the doc in the same working tree means it is being refreshed.
	writeFile(t, root, staleDoc,
		"---\ntitle: M\nstatus: current\nsummary: wip\nrelated_code:\n  - src/thing.py\n---\n# M\n")
	run(t, "sync", root)
	out, _ = checkOut(t, root)
	if hasStaleFinding(out, staleDoc) {
		t.Errorf("a doc edited alongside the code must not be stale, got:\n%s", out)
	}
}

func TestGitStalenessWarnsOnShallowClone(t *testing.T) {
	origin := gitRepoWithDoc(t)
	writeFile(t, origin, "src/thing.py", "print('changed')\n")
	commitAll(t, origin, "change code only")

	clone := filepath.Join(t.TempDir(), "clone")
	git(t, filepath.Dir(clone), "clone", "-q", "--depth", "1", "file://"+origin, clone)

	out, code := checkOut(t, clone)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d:\n%s", code, out)
	}
	if !hasStaleFinding(out, ".maat.yml") || !strings.Contains(out, "fetch-depth: 0") {
		t.Errorf("expected a shallow-history warning, got:\n%s", out)
	}
}

func TestStalenessSourceMtimeForcesHeuristic(t *testing.T) {
	root := gitRepoWithDoc(t)
	writeFile(t, root, ".maat.yml", "check:\n  staleness_source: mtime\n")
	commitAll(t, root, "force mtime")
	makeNewer(t, filepath.Join(root, "src/thing.py"))

	out, _ := checkOut(t, root)
	if !hasStaleFinding(out, staleDoc) || !strings.Contains(out, "mtime") {
		t.Errorf("staleness_source: mtime should use the mtime heuristic, got:\n%s", out)
	}
}

func TestStalenessSourceGitOutsideRepoErrors(t *testing.T) {
	root := initRepo(t) // plain temp dir, not a git work tree
	if out, err := exec.Command("git", "-C", root, "rev-parse", "--is-inside-work-tree").Output(); err == nil &&
		strings.TrimSpace(string(out)) == "true" {
		t.Skip("temp dir is inside a git work tree on this machine")
	}
	writeFile(t, root, ".maat.yml", "check:\n  staleness_source: git\n")
	out, code := checkOut(t, root)
	if code != 1 || !strings.Contains(out, "not inside a git work tree") {
		t.Errorf("expected an error finding (exit 1), got exit %d:\n%s", code, out)
	}
}

func TestStalenessSourceInvalidValue(t *testing.T) {
	root := initRepo(t)
	writeFile(t, root, ".maat.yml", "check:\n  staleness_source: svn\n")
	_, stderr, code := run(t, "check", root)
	if code != 2 || !strings.Contains(stderr, "staleness_source") {
		t.Errorf("expected config error (exit 2), got exit %d: %s", code, stderr)
	}
}

func TestStalenessOffSkipsGitEntirely(t *testing.T) {
	root := initRepo(t)
	// With staleness off, even a source that cannot work must not error.
	writeFile(t, root, ".maat.yml", "check:\n  staleness: off\n  staleness_source: git\n")
	if _, err := os.Stat(filepath.Join(root, ".git")); err == nil {
		t.Fatal("expected a plain directory")
	}
	run(t, "sync", root)
	out, code := checkOut(t, root)
	if code != 0 || strings.Contains(out, "[staleness]") {
		t.Errorf("staleness: off must skip the probe, got exit %d:\n%s", code, out)
	}
}
