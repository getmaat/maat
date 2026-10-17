package maat

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// stalenessProbe decides whether a doc lags behind one of its related_code
// paths. Paths are relative to the repository root that `maat check` runs on.
// See ADR 0012 for why git history is preferred over filesystem mtimes.
type stalenessProbe interface {
	isStale(codeRel, docRel string) bool
	// name is the source reported in findings ("git" or "mtime").
	name() string
}

// newStalenessProbe picks the probe for check.staleness_source:
//
//   - "mtime" always uses filesystem modification times (the ADR 0004 heuristic);
//   - "git" requires a git work tree and returns an error otherwise;
//   - "auto" (the default) uses git when root is inside a work tree and the git
//     binary is available, and falls back to mtimes otherwise.
//
// shallow reports whether the git history is truncated, so check can warn that
// commits beyond the clone depth are invisible to the probe.
func newStalenessProbe(root, source string) (probe stalenessProbe, shallow bool, err error) {
	if source == "mtime" {
		return mtimeProbe{root: root}, false, nil
	}
	if out, gErr := runGit(root, "rev-parse", "--is-inside-work-tree"); gErr == nil && out == "true" {
		shallowOut, _ := runGit(root, "rev-parse", "--is-shallow-repository")
		return newGitProbe(root), shallowOut == "true", nil
	}
	if source == "git" {
		return nil, false, fmt.Errorf("check.staleness_source is \"git\" but %s is not inside a git work tree (or git is not installed)", root)
	}
	return mtimeProbe{root: root}, false, nil
}

// mtimeProbe is the original ADR 0004 heuristic: a code file more than one
// second newer than its doc is stale. It works on any plain directory, but
// mtimes are meaningless after a fresh checkout (CI) and are reset by
// formatters, branch switches and rebases.
type mtimeProbe struct{ root string }

func (mtimeProbe) name() string { return "mtime" }

func (p mtimeProbe) isStale(codeRel, docRel string) bool {
	ci, err := os.Stat(filepath.Join(p.root, codeRel))
	if err != nil {
		return false
	}
	di, err := os.Stat(filepath.Join(p.root, docRel))
	if err != nil {
		return false
	}
	return ci.ModTime().Sub(di.ModTime()).Seconds() > 1.0
}

// gitProbe compares commit order instead of timestamps: a doc is stale when a
// commit reachable from HEAD touched the code after the last commit that touched
// the doc. A code change committed together with its doc is therefore never
// stale, which is exactly the "docs in the same change" protocol.
//
// Uncommitted work is taken into account: a doc with local edits counts as
// being refreshed right now, and code with local edits (while its doc has
// none) counts as newer than the doc.
type gitProbe struct {
	root       string
	dirty      map[string]bool
	lastCommit map[string]string
}

func newGitProbe(root string) *gitProbe {
	return &gitProbe{root: root, dirty: map[string]bool{}, lastCommit: map[string]string{}}
}

func (*gitProbe) name() string { return "git" }

func (p *gitProbe) isStale(codeRel, docRel string) bool {
	if p.isDirty(docRel) {
		return false
	}
	if p.isDirty(codeRel) {
		return true
	}
	docCommit := p.last(docRel)
	if docCommit == "" {
		return false
	}
	out, err := runGit(p.root, "rev-list", "-1", docCommit+"..HEAD", "--", codeRel)
	return err == nil && out != ""
}

// isDirty reports whether path (a file or directory) has staged, unstaged or
// untracked changes. Ignored files do not count.
func (p *gitProbe) isDirty(rel string) bool {
	if v, ok := p.dirty[rel]; ok {
		return v
	}
	out, err := runGit(p.root, "status", "--porcelain", "--untracked-files=all", "--", rel)
	v := err == nil && out != ""
	p.dirty[rel] = v
	return v
}

// last returns the hash of the newest commit reachable from HEAD that touched
// rel, or "" if there is none (untracked path, empty repository).
func (p *gitProbe) last(rel string) string {
	if v, ok := p.lastCommit[rel]; ok {
		return v
	}
	out, err := runGit(p.root, "log", "-1", "--format=%H", "--", rel)
	if err != nil {
		out = ""
	}
	p.lastCommit[rel] = out
	return out
}

// runGit runs git in dir and returns its trimmed stdout. Git is an optional,
// external tool — not a Go dependency — so ADR 0002/0011 are unaffected:
// without it, auto mode falls back to mtimes.
func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	// Never let git page or prompt (e.g. for credentials) inside check.
	cmd.Env = append(os.Environ(), "GIT_PAGER=cat", "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}
