package maat

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
)

// `maat contracts update [--check]` (ADR 0014 §2): fetch the manifests of
// consumed repositories and write .maat/contracts.lock.json, or with --check
// only report whether it is out of date.

// consumedByRepo groups this repository's cross-repo consumed ids by provider
// repository. Refs to the repository itself resolve locally and are skipped.
func consumedByRepo(model *DocsModel, cfg map[string]any) map[string][]string {
	self := configRepo(cfg)
	seen := map[string]map[string]bool{}
	for _, d := range collectContracts(model).Consumes {
		repo, id, cross := splitRef(d.Ref)
		if !cross || (self != "" && repo == self) {
			continue
		}
		if seen[repo] == nil {
			seen[repo] = map[string]bool{}
		}
		seen[repo][id] = true
	}
	out := map[string][]string{}
	for repo, ids := range seen {
		for id := range ids {
			out[repo] = append(out[repo], id)
		}
		sort.Strings(out[repo])
	}
	return out
}

// buildLock fetches every consumed repository's manifest and builds the new
// lockfile, or returns nil when nothing cross-repo is consumed. All sources
// are tried; their errors are returned together so one run shows every
// problem. old (may be nil) lets unchanged sources keep their entries.
func buildLock(model *DocsModel, cfg map[string]any, old *contractsLock) (*contractsLock, []error) {
	consumed := consumedByRepo(model, cfg)
	if len(consumed) == 0 {
		return nil, nil
	}
	sources := contractSources(cfg)
	repos := make([]string, 0, len(consumed))
	for repo := range consumed {
		repos = append(repos, repo)
	}
	sort.Strings(repos)

	var errs []error
	for _, repo := range repos {
		if _, ok := sources[repo]; !ok {
			errs = append(errs, fmt.Errorf("%s: no contracts.sources entry in %s", repo, configFilename))
		}
	}
	if len(errs) > 0 {
		return nil, errs
	}

	lock := &contractsLock{Schema: lockSchema, Sources: map[string]lockedSource{}}
	for _, repo := range repos {
		src := sources[repo]
		fetched, err := fetchManifest(model.Root, repo, src)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", repo, err))
			continue
		}
		entry := lockEntry(src, fetched, consumed[repo])
		// Keep the previous entry (and its commit) when nothing significant
		// changed, so provider commits alone never churn the lockfile.
		if old != nil {
			if prev, ok := old.Sources[repo]; ok && sameSignificant(prev, entry) {
				entry = prev
			}
		}
		lock.Sources[repo] = entry
	}
	if len(errs) > 0 {
		return nil, errs
	}
	return lock, nil
}

// lockEntry records the consumed ids a provider has, and those it lacks.
func lockEntry(src contractSource, f fetchedManifest, ids []string) lockedSource {
	provided := map[string]Contract{}
	for _, c := range f.manifest.Provides {
		provided[c.ID] = c
	}
	entry := lockedSource{Ref: src.Ref, URL: src.URL, Path: src.Path, Commit: f.commit,
		ManifestSchema: f.manifest.Schema, Provides: []lockedContract{}}
	for _, id := range ids {
		if c, ok := provided[id]; ok {
			entry.Provides = append(entry.Provides, lockedContract{ID: c.ID, Kind: c.Kind, Version: c.Version, Doc: c.Doc})
		} else {
			entry.Missing = append(entry.Missing, id)
		}
	}
	return entry
}

// sameSignificant compares what check acts on: the source settings, the
// manifest schema, which consumed ids exist, and their versions. Commit, kind
// and doc path are informational and do not count (ADR 0014, review 2).
func sameSignificant(a, b lockedSource) bool {
	strip := func(s lockedSource) lockedSource {
		s.Commit = ""
		p := make([]lockedContract, len(s.Provides))
		for i, c := range s.Provides {
			p[i] = lockedContract{ID: c.ID, Version: c.Version}
		}
		s.Provides = p
		if len(s.Missing) == 0 {
			s.Missing = nil
		}
		return s
	}
	return reflect.DeepEqual(strip(a), strip(b))
}

// renderLock serializes the lockfile deterministically.
func renderLock(l *contractsLock) string {
	out, err := json.MarshalIndent(l, "", "  ")
	if err != nil { // cannot happen for these plain types
		panic(err)
	}
	return string(out) + "\n"
}

// describeLockChanges lists, per provider, what differs between the lockfile
// on disk and the freshly built one. Either may be nil.
func describeLockChanges(old, updated *contractsLock) []string {
	oldSrc := map[string]lockedSource{}
	if old != nil {
		oldSrc = old.Sources
	}
	newSrc := map[string]lockedSource{}
	if updated != nil {
		newSrc = updated.Sources
	}
	var repos []string
	for r := range oldSrc {
		repos = append(repos, r)
	}
	for r := range newSrc {
		if _, ok := oldSrc[r]; !ok {
			repos = append(repos, r)
		}
	}
	sort.Strings(repos)

	var lines []string
	for _, repo := range repos {
		o, inOld := oldSrc[repo]
		n, inNew := newSrc[repo]
		switch {
		case !inOld:
			lines = append(lines, fmt.Sprintf("%s: not locked yet", repo))
		case !inNew:
			lines = append(lines, fmt.Sprintf("%s: no longer consumed", repo))
		case o.Ref != n.Ref || o.URL != n.URL || o.Path != n.Path:
			lines = append(lines, fmt.Sprintf("%s: source settings changed", repo))
		default:
			lines = append(lines, contractChanges(repo, o, n)...)
		}
	}
	return lines
}

func contractChanges(repo string, o, n lockedSource) []string {
	type state struct {
		present bool
		version string
	}
	states := func(s lockedSource) map[string]state {
		m := map[string]state{}
		for _, c := range s.Provides {
			m[c.ID] = state{true, c.Version}
		}
		for _, id := range s.Missing {
			m[id] = state{false, ""}
		}
		return m
	}
	was0, now0 := states(o), states(n)
	var ids []string
	for id := range now0 {
		ids = append(ids, id)
	}
	for id := range was0 {
		if _, ok := now0[id]; !ok {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	show := func(s state) string {
		if !s.present {
			return "not provided"
		}
		if s.version == "" {
			return "unversioned"
		}
		return s.version
	}
	var lines []string
	for _, id := range ids {
		was, hadOld := was0[id]
		now, hasNew := now0[id]
		switch {
		case !hadOld:
			lines = append(lines, fmt.Sprintf("%s#%s: newly consumed", repo, id))
		case !hasNew:
			lines = append(lines, fmt.Sprintf("%s#%s: no longer consumed", repo, id))
		case was != now:
			lines = append(lines, fmt.Sprintf("%s#%s: %s → %s", repo, id, show(was), show(now)))
		}
	}
	if len(lines) == 0 && o.ManifestSchema != n.ManifestSchema {
		lines = append(lines, fmt.Sprintf("%s: manifest schema %d → %d", repo, o.ManifestSchema, n.ManifestSchema))
	}
	return lines
}

// cmdContracts dispatches `maat contracts <subcommand>`.
func cmdContracts(args []string, stdout, stderr io.Writer) (int, error) {
	if len(args) == 0 || args[0] != "update" {
		return 0, &usageError{"usage: maat contracts update [--check] [PATH]"}
	}
	p, err := parseArgs("contracts update", args[1:])
	if err != nil {
		return 0, err
	}
	root := absRoot(p.path)
	cfg, err := LoadConfig(root)
	if err != nil {
		return 0, err
	}
	if err := enforceVersion(cfg); err != nil {
		return 0, err
	}
	docsDir := AnyToStr(cfg["docs_dir"])
	if info, err := os.Stat(filepath.Join(root, docsDir)); err != nil || !info.IsDir() {
		fmt.Fprintf(stderr, "No %s/ directory found. Run `maat init` first.\n", docsDir)
		return 2, nil
	}
	model, err := ScanModel(root, docsDir)
	if err != nil {
		return 0, err
	}

	old, _, _ := readLock(root) // an unreadable lockfile is simply replaced
	updated, errs := buildLock(model, cfg, old)
	if len(errs) > 0 {
		fmt.Fprintln(stderr, "maat: contracts update failed — the lockfile was not changed:")
		for _, e := range errs {
			fmt.Fprintf(stderr, "  - %s\n", e)
		}
		return 1, nil
	}

	lockFile := filepath.Join(root, lockPath)
	current := readFileOrEmpty(lockFile)
	want := ""
	if updated != nil {
		want = renderLock(updated)
	}
	upToDate := normalizeText(current) == normalizeText(want)

	if p.checkLock {
		if upToDate {
			fmt.Fprintln(stdout, "Contracts lockfile is up to date.")
			return 0, nil
		}
		fmt.Fprintf(stdout, "%s is out of date:\n", lockPath)
		changes := describeLockChanges(old, updated)
		if len(changes) == 0 {
			changes = []string{"the file differs from what `maat contracts update` would write"}
		}
		for _, c := range changes {
			fmt.Fprintf(stdout, "  - %s\n", c)
		}
		fmt.Fprintln(stdout, "\nRun `maat contracts update` and commit the result.")
		return 1, nil
	}

	switch {
	case updated == nil && current == "":
		fmt.Fprintln(stdout, "No cross-repo contracts are consumed — nothing to lock.")
	case upToDate:
		fmt.Fprintln(stdout, "Contracts lockfile is already up to date.")
	case updated == nil:
		if err := os.Remove(lockFile); err != nil {
			return 0, err
		}
		fmt.Fprintf(stdout, "  remove  %s (no cross-repo contracts are consumed)\n", lockPath)
	default:
		for _, c := range describeLockChanges(old, updated) {
			fmt.Fprintf(stdout, "  - %s\n", c)
		}
		if err := writeFileMkdir(lockFile, want); err != nil {
			return 0, err
		}
		fmt.Fprintf(stdout, "  update  %s\n", lockPath)
	}
	return 0, nil
}

// writeFileMkdir writes content, creating parent directories as needed.
func writeFileMkdir(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}
