package maat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --------------------------------------------------------------------------- #
// helpers
// --------------------------------------------------------------------------- #

// run invokes the CLI with the given args, capturing stdout+stderr and the exit
// code. It is the Go analogue of the Python tests' capsys-based main() driver.
func run(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	var out, errb strings.Builder
	code := Main(args, &out, &errb)
	return out.String(), errb.String(), code
}

// initRepo scaffolds a fresh Ma'at repo in a temp dir and returns its root.
func initRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	_, _, code := run(t, "init", root, "--name", "TestProj", "--summary", "A test project.")
	if code != 0 {
		t.Fatalf("init exited %d", code)
	}
	return root
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// makeNewer sets a file's mtime well into the future so staleness fires
// deterministically regardless of filesystem timestamp granularity.
func makeNewer(t *testing.T, path string) {
	t.Helper()
	future := time.Now().Add(10 * time.Second)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatal(err)
	}
}

// --------------------------------------------------------------------------- #
// init
// --------------------------------------------------------------------------- #

func TestInitCreatesScaffold(t *testing.T) {
	root := initRepo(t)
	for _, rel := range []string{
		"AGENTS.md",
		"docs/index.md",
		"docs/llms.txt",
		".maat.yml",
		"CLAUDE.md",
		".github/copilot-instructions.md",
		".cursor/rules/maat.mdc",
		".hermes.md",
	} {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Errorf("expected %s to exist: %v", rel, err)
		}
	}
}

func TestInitIsIdempotent(t *testing.T) {
	root := initRepo(t)
	before := readFile(t, root, "AGENTS.md")
	// Hand-edit AGENTS.md, then re-init without --force: must be preserved.
	writeFile(t, root, "AGENTS.md", before+"\nHAND EDIT\n")
	out, _, code := run(t, "init", root, "--name", "TestProj")
	if code != 0 {
		t.Fatalf("re-init exited %d", code)
	}
	if !strings.Contains(readFile(t, root, "AGENTS.md"), "HAND EDIT") {
		t.Error("re-init clobbered a hand-edited AGENTS.md")
	}
	if !strings.Contains(out, "skip") {
		t.Error("expected skip lines on re-init")
	}
}

func TestInitForceOverwrites(t *testing.T) {
	root := initRepo(t)
	writeFile(t, root, "AGENTS.md", "HAND EDIT\n")
	if _, _, code := run(t, "init", root, "--name", "TestProj", "--force"); code != 0 {
		t.Fatalf("force init exited %d", code)
	}
	if strings.Contains(readFile(t, root, "AGENTS.md"), "HAND EDIT") {
		t.Error("--force should have overwritten AGENTS.md")
	}
}

// --------------------------------------------------------------------------- #
// sync
// --------------------------------------------------------------------------- #

func TestSyncIsIdempotent(t *testing.T) {
	root := initRepo(t)
	out, _, code := run(t, "sync", root)
	if code != 0 {
		t.Fatalf("sync exited %d", code)
	}
	if !strings.Contains(out, "Already in sync") {
		t.Errorf("expected no-op sync, got: %s", out)
	}
}

func TestSyncRegeneratesDrift(t *testing.T) {
	root := initRepo(t)
	// Corrupt llms.txt; sync must restore it.
	writeFile(t, root, "docs/llms.txt", "GARBAGE\n")
	out, _, code := run(t, "sync", root)
	if code != 0 {
		t.Fatalf("sync exited %d", code)
	}
	if !strings.Contains(out, "update") || !strings.Contains(out, "docs/llms.txt") {
		t.Errorf("expected llms.txt to be regenerated, got: %s", out)
	}
	if strings.Contains(readFile(t, root, "docs/llms.txt"), "GARBAGE") {
		t.Error("sync did not overwrite corrupted llms.txt")
	}
}

// --------------------------------------------------------------------------- #
// check: clean
// --------------------------------------------------------------------------- #

func TestCheckCleanScaffold(t *testing.T) {
	root := initRepo(t)
	out, _, code := run(t, "check", root)
	if code != 0 {
		t.Fatalf("check on clean scaffold exited %d:\n%s", code, out)
	}
	if !strings.Contains(out, "0 error(s)") {
		t.Errorf("expected 0 errors, got: %s", out)
	}
}

func TestCheckWithoutDocsExits2(t *testing.T) {
	root := t.TempDir()
	_, errOut, code := run(t, "check", root)
	if code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
	if !strings.Contains(errOut, "Run `maat init` first") {
		t.Errorf("expected init hint on stderr, got: %s", errOut)
	}
}

// --------------------------------------------------------------------------- #
// check: drift
// --------------------------------------------------------------------------- #

func TestCheckDetectsDrift(t *testing.T) {
	root := initRepo(t)
	writeFile(t, root, "docs/llms.txt", "GARBAGE\n")
	out, _, code := run(t, "check", root)
	if code != 1 {
		t.Fatalf("expected exit 1 on drift, got %d", code)
	}
	if !strings.Contains(out, "drift") {
		t.Errorf("expected drift finding, got: %s", out)
	}
}

// --------------------------------------------------------------------------- #
// check: front-matter
// --------------------------------------------------------------------------- #

func TestCheckMissingRequiredFrontmatter(t *testing.T) {
	root := initRepo(t)
	writeFile(t, root, "docs/architecture/modules/m.md",
		"---\nsummary: no title or status\n---\n# M\n")
	out, _, code := run(t, "check", root)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(out, "frontmatter") {
		t.Errorf("expected frontmatter finding, got: %s", out)
	}
}

func TestCheckUnknownStatus(t *testing.T) {
	root := initRepo(t)
	writeFile(t, root, "docs/architecture/modules/m.md",
		"---\ntitle: M\nstatus: bogus\nsummary: x\n---\n# M\n")
	out, _, code := run(t, "check", root)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(out, "status") {
		t.Errorf("expected status finding, got: %s", out)
	}
}

// --------------------------------------------------------------------------- #
// check: links & related_code
// --------------------------------------------------------------------------- #

func TestCheckBrokenLink(t *testing.T) {
	root := initRepo(t)
	writeFile(t, root, "docs/architecture/modules/m.md",
		"---\ntitle: M\nstatus: current\nsummary: x\n---\n# M\nSee [gone](./nope.md).\n")
	out, _, code := run(t, "check", root)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(out, "broken_link") {
		t.Errorf("expected broken_link finding, got: %s", out)
	}
}

func TestCheckOrphanedCode(t *testing.T) {
	root := initRepo(t)
	writeFile(t, root, "docs/architecture/modules/m.md",
		"---\ntitle: M\nstatus: current\nsummary: x\nrelated_code:\n  - src/nope.py\n---\n# M\n")
	out, _, code := run(t, "check", root)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(out, "orphaned_code") {
		t.Errorf("expected orphaned_code finding, got: %s", out)
	}
}

// --------------------------------------------------------------------------- #
// check: staleness
// --------------------------------------------------------------------------- #

func TestCheckStalenessWarnsByDefault(t *testing.T) {
	root := initRepo(t)
	writeFile(t, root, "src/thing.py", "print('hi')\n")
	writeFile(t, root, "docs/architecture/modules/m.md",
		"---\ntitle: M\nstatus: current\nsummary: x\nrelated_code:\n  - src/thing.py\n---\n# M\n")
	// Clear drift first, then make code newer than the doc.
	run(t, "sync", root)
	makeNewer(t, filepath.Join(root, "src/thing.py"))

	out, _, code := run(t, "check", root)
	if code != 0 {
		t.Fatalf("staleness should warn (exit 0), got %d:\n%s", code, out)
	}
	if !strings.Contains(out, "staleness") {
		t.Errorf("expected staleness warning, got: %s", out)
	}
}

func TestCheckStrictPromotesStaleness(t *testing.T) {
	root := initRepo(t)
	writeFile(t, root, "src/thing.py", "print('hi')\n")
	writeFile(t, root, "docs/architecture/modules/m.md",
		"---\ntitle: M\nstatus: current\nsummary: x\nrelated_code:\n  - src/thing.py\n---\n# M\n")
	run(t, "sync", root)
	makeNewer(t, filepath.Join(root, "src/thing.py"))

	_, _, code := run(t, "check", root, "--strict")
	if code != 1 {
		t.Fatalf("--strict should promote staleness to error (exit 1), got %d", code)
	}
}

func TestCheckIgnoreCodePathsSuppressesStaleness(t *testing.T) {
	root := initRepo(t)
	writeFile(t, root, "vendor/lib.py", "x = 1\n")
	writeFile(t, root, "docs/architecture/modules/v.md",
		"---\ntitle: V\nstatus: current\nsummary: x\nrelated_code:\n  - vendor/lib.py\n---\n# V\n")
	writeFile(t, root, ".maat.yml", "check:\n  ignore_code_paths:\n    - vendor/\n")
	run(t, "sync", root)
	makeNewer(t, filepath.Join(root, "vendor/lib.py"))

	out, _, code := run(t, "check", root)
	if code != 0 {
		t.Fatalf("ignored path should not error, got %d:\n%s", code, out)
	}
	if strings.Contains(out, "staleness") {
		t.Errorf("ignore_code_paths should suppress staleness, got: %s", out)
	}
}

// --------------------------------------------------------------------------- #
// check: partial config merge
// --------------------------------------------------------------------------- #

func TestPartialConfigMergePreservesDefaults(t *testing.T) {
	root := initRepo(t)
	writeFile(t, root, "src/thing.py", "print('hi')\n")
	writeFile(t, root, "docs/architecture/modules/m.md",
		"---\ntitle: M\nstatus: current\nsummary: x\nrelated_code:\n  - src/thing.py\n---\n# M\n")
	// Override only drift_is_error; staleness default (warn) must survive.
	writeFile(t, root, ".maat.yml", "check:\n  drift_is_error: false\n")
	run(t, "sync", root)
	makeNewer(t, filepath.Join(root, "src/thing.py"))

	out, _, code := run(t, "check", root)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d:\n%s", code, out)
	}
	if !strings.Contains(out, "staleness") {
		t.Error("partial config replaced sibling defaults — staleness default lost")
	}
}

// --------------------------------------------------------------------------- #
// check: output format
// --------------------------------------------------------------------------- #

func TestCheckGitHubFormat(t *testing.T) {
	root := initRepo(t)
	writeFile(t, root, "docs/llms.txt", "GARBAGE\n")
	out, _, code := run(t, "check", root, "--format", "github")
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(out, "::error ") || !strings.Contains(out, "title=maat drift") {
		t.Errorf("expected GitHub annotation, got: %s", out)
	}
}

// --------------------------------------------------------------------------- #
// config validation
// --------------------------------------------------------------------------- #

func TestCheckUnknownAdapterErrors(t *testing.T) {
	root := initRepo(t)
	writeFile(t, root, ".maat.yml", "adapters:\n  - claude\n  - bogus\n")
	_, errOut, code := run(t, "check", root)
	if code != 2 {
		t.Fatalf("expected exit 2 for bad config, got %d", code)
	}
	if !strings.Contains(errOut, "unknown adapter") {
		t.Errorf("expected unknown-adapter error, got: %s", errOut)
	}
}

func TestCheckBadStalenessValueErrors(t *testing.T) {
	root := initRepo(t)
	writeFile(t, root, ".maat.yml", "check:\n  staleness: sometimes\n")
	_, errOut, code := run(t, "check", root)
	if code != 2 {
		t.Fatalf("expected exit 2 for bad staleness, got %d", code)
	}
	if !strings.Contains(errOut, "staleness") {
		t.Errorf("expected staleness validation error, got: %s", errOut)
	}
}

// --------------------------------------------------------------------------- #
// version
// --------------------------------------------------------------------------- #

func TestVersionFlag(t *testing.T) {
	out, _, code := run(t, "--version")
	if code != 0 {
		t.Fatalf("--version exited %d", code)
	}
	if !strings.Contains(out, "maat "+Version) {
		t.Errorf("expected version string, got: %s", out)
	}
}
