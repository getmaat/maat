package maat

import (
	"io"
	"strings"
	"testing"
)

// --------------------------------------------------------------------------- #
// init wizard trigger (interactive TTY + no --name/--summary)
// --------------------------------------------------------------------------- #

func TestCmdInitSkipsWizardWhenNotATTY(t *testing.T) {
	// No overrides at all: the run() helper's strings.Builder is never a
	// *os.File, so isInteractiveTerminal is false by construction — this
	// proves the wizard branch is unreachable from the existing test suite.
	root := t.TempDir()
	_, _, code := run(t, "init", root)
	if code != 0 {
		t.Fatalf("init exited %d", code)
	}
}

func TestCmdInitLaunchesWizardWhenInteractiveAndNoFlags(t *testing.T) {
	oldTTY, oldWizard := isInteractiveTerminal, runInitWizard
	defer func() { isInteractiveTerminal, runInitWizard = oldTTY, oldWizard }()

	isInteractiveTerminal = func(io.Writer) bool { return true }
	called := false
	runInitWizard = func(defaultName string, defaultAgents []string) (wizardResult, error) {
		called = true
		return wizardResult{name: "Wizarded", summary: "from the wizard", agents: defaultAgents, ok: true}, nil
	}

	root := t.TempDir()
	_, _, code := run(t, "init", root)
	if code != 0 {
		t.Fatalf("init exited %d", code)
	}
	if !called {
		t.Fatal("expected runInitWizard to be called")
	}
	if !strings.Contains(readFile(t, root, "AGENTS.md"), "Wizarded") {
		t.Error("expected wizard-supplied name to reach the scaffold")
	}
}

func TestCmdInitSkipsWizardWhenFlagsGiven(t *testing.T) {
	oldTTY, oldWizard := isInteractiveTerminal, runInitWizard
	defer func() { isInteractiveTerminal, runInitWizard = oldTTY, oldWizard }()

	isInteractiveTerminal = func(io.Writer) bool { return true } // even though "interactive"...
	runInitWizard = func(defaultName string, defaultAgents []string) (wizardResult, error) {
		t.Fatal("wizard must not be invoked when --name/--summary are given")
		return wizardResult{}, nil
	}

	root := t.TempDir()
	_, _, code := run(t, "init", root, "--name", "Flagged", "--summary", "from flags")
	if code != 0 {
		t.Fatalf("init exited %d", code)
	}
}

func TestCmdInitReturns130OnWizardAbort(t *testing.T) {
	oldTTY, oldWizard := isInteractiveTerminal, runInitWizard
	defer func() { isInteractiveTerminal, runInitWizard = oldTTY, oldWizard }()

	isInteractiveTerminal = func(io.Writer) bool { return true }
	runInitWizard = func(defaultName string, defaultAgents []string) (wizardResult, error) {
		return wizardResult{ok: false}, nil
	}

	root := t.TempDir()
	_, _, code := run(t, "init", root)
	if code != 130 {
		t.Fatalf("expected exit code 130 on wizard abort, got %d", code)
	}
}

// --------------------------------------------------------------------------- #
// --agents selection
// --------------------------------------------------------------------------- #

func mustNotExist(t *testing.T, root, rel string) {
	t.Helper()
	if pathExists(root + "/" + rel) {
		t.Errorf("expected %s not to be generated", rel)
	}
}

func mustExist(t *testing.T, root, rel string) {
	t.Helper()
	if !pathExists(root + "/" + rel) {
		t.Errorf("expected %s to be generated", rel)
	}
}

func TestCmdInitAgentsFlagSelectsSubset(t *testing.T) {
	root := t.TempDir()
	_, _, code := run(t, "init", root, "--name", "X", "--summary", "Y", "--agents", "claude,cursor")
	if code != 0 {
		t.Fatalf("init exited %d", code)
	}
	mustExist(t, root, "CLAUDE.md")
	mustExist(t, root, ".cursor/rules/maat.mdc")
	mustNotExist(t, root, ".hermes.md")
	mustNotExist(t, root, "GEMINI.md")
	mustNotExist(t, root, ".windsurf/rules/maat.md")
	mustNotExist(t, root, ".github/copilot-instructions.md")

	cfg, err := LoadConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := toStringList(cfg["adapters"]); len(got) != 2 || got[0] != "claude" || got[1] != "cursor" {
		t.Errorf("expected adapters: [claude cursor], got %v", got)
	}
}

func TestCmdInitAgentsFlagAllShorthand(t *testing.T) {
	root := t.TempDir()
	_, _, code := run(t, "init", root, "--name", "X", "--summary", "Y", "--agents=all")
	if code != 0 {
		t.Fatalf("init exited %d", code)
	}
	for _, rel := range []string{"CLAUDE.md", ".hermes.md", "GEMINI.md", ".windsurf/rules/maat.md", ".github/copilot-instructions.md", ".cursor/rules/maat.mdc"} {
		mustExist(t, root, rel)
	}
}

func TestCmdInitAgentsFlagEmptyMeansNone(t *testing.T) {
	root := t.TempDir()
	_, _, code := run(t, "init", root, "--name", "X", "--summary", "Y", "--agents=")
	if code != 0 {
		t.Fatalf("init exited %d", code)
	}
	for _, rel := range []string{"CLAUDE.md", ".hermes.md", "GEMINI.md", ".windsurf/rules/maat.md", ".github/copilot-instructions.md", ".cursor/rules/maat.mdc"} {
		mustNotExist(t, root, rel)
	}
	mustExist(t, root, "AGENTS.md")
}

func TestCmdInitAgentsFlagRejectsUnknown(t *testing.T) {
	root := t.TempDir()
	_, errOut, code := run(t, "init", root, "--name", "X", "--summary", "Y", "--agents", "bogus")
	if code != 2 {
		t.Fatalf("expected exit 2 for an unknown agent, got %d", code)
	}
	if !strings.Contains(errOut, "unknown adapter") {
		t.Errorf("expected an unknown-adapter error, got: %s", errOut)
	}
}

func TestCmdInitWizardAgentSelectionReachesConfig(t *testing.T) {
	oldTTY, oldWizard := isInteractiveTerminal, runInitWizard
	defer func() { isInteractiveTerminal, runInitWizard = oldTTY, oldWizard }()

	isInteractiveTerminal = func(io.Writer) bool { return true }
	runInitWizard = func(defaultName string, defaultAgents []string) (wizardResult, error) {
		return wizardResult{name: defaultName, agents: []string{"hermes"}, ok: true}, nil
	}

	root := t.TempDir()
	_, _, code := run(t, "init", root)
	if code != 0 {
		t.Fatalf("init exited %d", code)
	}
	mustExist(t, root, ".hermes.md")
	mustNotExist(t, root, "CLAUDE.md")
}

func TestCmdInitHintsAboutMissingAgentOnReinit(t *testing.T) {
	root := t.TempDir()
	if _, _, code := run(t, "init", root, "--name", "X", "--summary", "Y", "--agents", "claude"); code != 0 {
		t.Fatalf("first init exited %d", code)
	}
	out, _, code := run(t, "init", root, "--name", "X", "--summary", "Y", "--agents", "claude,windsurf")
	if code != 0 {
		t.Fatalf("second init exited %d", code)
	}
	if !strings.Contains(out, "windsurf") || !strings.Contains(out, "maat sync") {
		t.Errorf("expected a hint about adding windsurf + running maat sync, got: %s", out)
	}
	// .maat.yml is a scaffold file, preserved untouched — still just [claude].
	cfg, err := LoadConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := toStringList(cfg["adapters"]); len(got) != 1 || got[0] != "claude" {
		t.Errorf("expected .maat.yml to be untouched ([claude]), got %v", got)
	}
	mustNotExist(t, root, ".windsurf/rules/maat.md")
}

// --------------------------------------------------------------------------- #
// color styling seam
// --------------------------------------------------------------------------- #

func TestColorPathPreservesPlainText(t *testing.T) {
	oldColor := isColorEnabled
	defer func() { isColorEnabled = oldColor }()
	isColorEnabled = func(io.Writer) bool { return true }

	root := t.TempDir()
	out, _, code := run(t, "init", root, "--name", "X", "--summary", "Y")
	if code != 0 {
		t.Fatalf("init exited %d", code)
	}
	if !strings.Contains(out, "\x1b[") {
		t.Error("expected ANSI escape codes when color is forced on")
	}
	plain := stripANSI(out)
	if !strings.Contains(plain, "  create  ") {
		t.Errorf("stripped output should still read like the plain path, got: %q", plain)
	}
}

func TestCheckColorPathIsNeverAppliedToGitHubFormat(t *testing.T) {
	oldColor := isColorEnabled
	defer func() { isColorEnabled = oldColor }()
	isColorEnabled = func(io.Writer) bool { return true }

	root := initRepo(t)
	out, _, _ := run(t, "check", root, "--format", "github")
	if strings.Contains(out, "\x1b[") {
		t.Error("--format=github output must never contain ANSI escape codes")
	}
}

func stripANSI(s string) string {
	var b strings.Builder
	inEscape := false
	for _, r := range s {
		switch {
		case r == '\x1b':
			inEscape = true
		case inEscape && r == 'm':
			inEscape = false
		case !inEscape:
			b.WriteRune(r)
		}
	}
	return b.String()
}
