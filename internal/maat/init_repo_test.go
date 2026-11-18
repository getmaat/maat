package maat

import (
	"os/exec"
	"strings"
	"testing"
)

// `maat init` pre-fills repo: from the git origin remote once (ADR 0013 §2).

func TestRepoFromURL(t *testing.T) {
	cases := map[string]string{
		"https://github.com/getmaat/maat.git":     "getmaat/maat",
		"https://github.com/getmaat/maat":         "getmaat/maat",
		"https://github.com/getmaat/maat/":        "getmaat/maat",
		"https://user@gitlab.example.com/a/b.git": "a/b",
		"git@github.com:getmaat/maat.git":         "getmaat/maat",
		"git@github.com:getmaat/maat":             "getmaat/maat",
		"ssh://git@github.com/getmaat/maat.git":   "getmaat/maat",
		"ssh://git@host.example:2222/o/r.git":     "o/r",
		"https://gitlab.com/group/sub/repo.git":   "", // subgroup: not owner/name
		"/srv/git/repo.git":                       "", // local path
		"file:///srv/git/repo.git":                "",
		"":                                        "",
	}
	for in, want := range cases {
		if got := repoFromURL(in); got != want {
			t.Errorf("repoFromURL(%q) = %q, want %q", in, got, want)
		}
	}
}

// gitDirWithRemote returns an empty git work tree whose origin is url.
func gitDirWithRemote(t *testing.T, url string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	root := t.TempDir()
	git(t, root, "init", "-q")
	if url != "" {
		git(t, root, "remote", "add", "origin", url)
	}
	return root
}

func TestInitPrefillsRepoFromOrigin(t *testing.T) {
	root := gitDirWithRemote(t, "git@github.com:acme/billing.git")
	out, _, code := run(t, "init", root, "--name", "P", "--summary", "S.")
	if code != 0 {
		t.Fatalf("init exited %d:\n%s", code, out)
	}
	cfg := readFile(t, root, ".maat.yml")
	if !strings.Contains(cfg, "\nrepo: acme/billing\n") {
		t.Errorf(".maat.yml should set repo from origin:\n%s", cfg)
	}
	if !strings.Contains(out, "repo: acme/billing") || !strings.Contains(out, "check it before committing") {
		t.Errorf("init should say where repo came from:\n%s", out)
	}
	if m := readManifest(t, root); m.Repo == nil || *m.Repo != "acme/billing" {
		t.Errorf("manifest repo = %v, want acme/billing", m.Repo)
	}
	if _, code := checkOut(t, root); code != 0 {
		t.Errorf("a freshly initialized repo should pass check, got exit %d", code)
	}
}

func TestInitLeavesRepoCommentedWithoutUsableOrigin(t *testing.T) {
	for name, url := range map[string]string{
		"no git":       "-",
		"no remote":    "",
		"subgroup url": "https://gitlab.com/group/sub/repo.git",
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			if url != "-" {
				root = gitDirWithRemote(t, url)
			}
			out, _, code := run(t, "init", root, "--name", "P", "--summary", "S.")
			if code != 0 {
				t.Fatalf("init exited %d", code)
			}
			cfg := readFile(t, root, ".maat.yml")
			if !strings.Contains(cfg, "\n# repo: owner/name\n") || strings.Contains(cfg, "\nrepo:") {
				t.Errorf("expected a commented repo placeholder:\n%s", cfg)
			}
			if strings.Contains(out, "from the git origin remote") {
				t.Errorf("init must not claim a repo it did not set:\n%s", out)
			}
		})
	}
}

func TestInitNeverRewritesExistingConfigRepo(t *testing.T) {
	root := gitDirWithRemote(t, "https://github.com/acme/fork.git")
	writeFile(t, root, ".maat.yml", "repo: acme/upstream\n")
	out, _, code := run(t, "init", root, "--name", "P", "--summary", "S.")
	if code != 0 {
		t.Fatalf("init exited %d", code)
	}
	if cfg := readFile(t, root, ".maat.yml"); cfg != "repo: acme/upstream\n" {
		t.Errorf("an existing .maat.yml must be left alone, got:\n%s", cfg)
	}
	if strings.Contains(out, "from the git origin remote") {
		t.Errorf("init must not report a repo it did not write:\n%s", out)
	}
}
