package maat

import (
	"io"
	"strings"
	"testing"
)

func TestInsertBadge(t *testing.T) {
	cases := map[string]struct{ in, want string }{
		"below title": {
			in:   "# Project\n\nSome text.\n",
			want: "# Project\n\n" + readmeBadge + "\n\nSome text.\n",
		},
		"title without blank line": {
			in:   "# Project\nSome text.\n",
			want: "# Project\n\n" + readmeBadge + "\n\nSome text.\n",
		},
		"title only": {
			in:   "# Project\n",
			want: "# Project\n\n" + readmeBadge + "\n",
		},
		"no title: prepend": {
			in:   "<p align=\"center\">Logo</p>\n",
			want: readmeBadge + "\n\n<p align=\"center\">Logo</p>\n",
		},
		"heading far down is a section, not the title": {
			in:   strings.Repeat("text\n", headingSearchLines) + "# Section\n",
			want: readmeBadge + "\n\n" + strings.Repeat("text\n", headingSearchLines) + "# Section\n",
		},
	}
	for name, c := range cases {
		if got := insertBadge(c.in); got != c.want {
			t.Errorf("%s:\ngot:\n%q\nwant:\n%q", name, got, c.want)
		}
	}
}

func TestAddReadmeBadgeIsIdempotent(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "README.md", "# P\n\nText.\n")
	if added, err := addReadmeBadge(root); err != nil || !added {
		t.Fatalf("first add: added=%v err=%v", added, err)
	}
	first := readFile(t, root, "README.md")
	if added, err := addReadmeBadge(root); err != nil || added {
		t.Fatalf("second add: added=%v err=%v", added, err)
	}
	if got := readFile(t, root, "README.md"); got != first || strings.Count(got, badgeMarker) != 1 {
		t.Errorf("second add changed the README:\n%s", got)
	}
}

func TestAddReadmeBadgeNeverCreatesAReadme(t *testing.T) {
	root := t.TempDir()
	if _, err := addReadmeBadge(root); err == nil {
		t.Fatal("expected an error when README.md does not exist")
	}
	if exists, _ := readmeBadgeState(root); exists {
		t.Error("addReadmeBadge must not create README.md")
	}
}

// stubWizard makes init interactive and answers the wizard with badge=answer,
// recording whether the badge question was offered.
func stubWizard(t *testing.T, answer bool) *bool {
	t.Helper()
	oldTTY, oldColor, oldWizard := isInteractiveTerminal, isColorEnabled, runInitWizard
	t.Cleanup(func() { isInteractiveTerminal, isColorEnabled, runInitWizard = oldTTY, oldColor, oldWizard })
	isInteractiveTerminal = func(io.Writer) bool { return true }
	isColorEnabled = func(io.Writer) bool { return false } // assert on plain text
	offered := false
	runInitWizard = func(defaultName string, defaultAgents []string, offerBadge bool) (wizardResult, error) {
		offered = offerBadge
		return wizardResult{name: "P", agents: defaultAgents, badge: offerBadge && answer, ok: true}, nil
	}
	return &offered
}

func TestInitWizardAddsBadgeWhenAccepted(t *testing.T) {
	offered := stubWizard(t, true)
	root := t.TempDir()
	writeFile(t, root, "README.md", "# P\n\nText.\n")
	out, _, code := run(t, "init", root)
	if code != 0 {
		t.Fatalf("init exited %d:\n%s", code, out)
	}
	if !*offered {
		t.Fatal("the badge question should be offered when README.md has no badge")
	}
	if !strings.Contains(readFile(t, root, "README.md"), readmeBadge) {
		t.Error("README.md should carry the badge")
	}
	if !strings.Contains(out, "  update  README.md") || !strings.Contains(out, "Added a docs badge") {
		t.Errorf("init should report the README change:\n%s", out)
	}
}

func TestInitWizardLeavesReadmeWhenDeclined(t *testing.T) {
	stubWizard(t, false)
	root := t.TempDir()
	writeFile(t, root, "README.md", "# P\n")
	out, _, code := run(t, "init", root)
	if code != 0 {
		t.Fatalf("init exited %d:\n%s", code, out)
	}
	if got := readFile(t, root, "README.md"); got != "# P\n" {
		t.Errorf("README.md must be untouched when declined:\n%s", got)
	}
	if !strings.Contains(out, "Optional: show readers") {
		t.Errorf("init should still print the badge hint:\n%s", out)
	}
}

func TestInitWizardDoesNotOfferBadgeWithoutReadmeOrWhenPresent(t *testing.T) {
	offered := stubWizard(t, true)

	noReadme := t.TempDir()
	if _, _, code := run(t, "init", noReadme); code != 0 {
		t.Fatalf("init exited %d", code)
	}
	if *offered {
		t.Error("no README.md: the badge question must not be offered")
	}
	if exists, _ := readmeBadgeState(noReadme); exists {
		t.Error("init must never create README.md")
	}

	withBadge := t.TempDir()
	writeFile(t, withBadge, "README.md", "# P\n\n"+readmeBadge+"\n")
	out, _, code := run(t, "init", withBadge)
	if code != 0 {
		t.Fatalf("init exited %d", code)
	}
	if *offered {
		t.Error("README.md already has the badge: the question must not be offered")
	}
	if strings.Contains(out, "Optional: show readers") {
		t.Errorf("no hint when the badge is already there:\n%s", out)
	}
}

func TestInitNonInteractiveNeverTouchesReadme(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "README.md", "# P\n")
	out, _, code := run(t, "init", root, "--name", "P", "--summary", "S.")
	if code != 0 {
		t.Fatalf("init exited %d", code)
	}
	if got := readFile(t, root, "README.md"); got != "# P\n" {
		t.Errorf("non-interactive init must not edit README.md:\n%s", got)
	}
	if !strings.Contains(out, readmeBadge) {
		t.Errorf("non-interactive init should print the badge snippet:\n%s", out)
	}
}
