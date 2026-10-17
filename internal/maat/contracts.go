package maat

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Contract edges (ADR 0013): docs declare the contracts they describe
// (`provides`) and the contracts they depend on (`consumes`). This file parses
// and validates those declarations and renders the generated manifest.

// manifestPath is the fixed, discoverable location of the generated manifest.
// It lives under .maat/ (not docs_dir, which is configurable) so other repos
// and tools can always find it (ADR 0013 §3).
const manifestPath = ".maat/manifest.json"

// manifestSchema versions the manifest format for its consumers.
const manifestSchema = 1

var (
	contractIDRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	repoNameRe   = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)
	providesKeys = []string{"id", "kind", "version", "spec"}
	consumesKeys = []string{"ref", "version"}
)

// Contract is one validated `provides` entry.
type Contract struct {
	ID      string `json:"id"`
	Kind    string `json:"kind,omitempty"`
	Version string `json:"version,omitempty"`
	Spec    string `json:"spec,omitempty"`
	Doc     string `json:"doc"`
}

// Dependency is one validated `consumes` entry.
type Dependency struct {
	Ref     string `json:"ref"`
	Version string `json:"version,omitempty"`
	Doc     string `json:"doc"`
}

// local reports whether the dependency points at a contract in this repo
// (a bare id, without an "owner/repo#" prefix).
func (d Dependency) local() bool { return !strings.Contains(d.Ref, "#") }

// contractSet is every valid edge in the docs tree, plus the problems found
// while collecting them. Invalid entries are reported and left out, so one
// typo does not hide the rest of the manifest.
type contractSet struct {
	Provides []Contract
	Consumes []Dependency
	problems []Finding // contract_schema findings, severity filled in by check
}

// collectContracts reads provides/consumes from every document in the model.
func collectContracts(model *DocsModel) contractSet {
	var cs contractSet
	seen := map[string]string{} // contract id -> doc that first declared it
	for _, doc := range model.Documents {
		for _, c := range parseProvides(doc, &cs) {
			if first, dup := seen[c.ID]; dup {
				cs.problem(doc.Rel, "duplicate contract id %s (already provided by %s)", pyRepr(c.ID), first)
				continue
			}
			seen[c.ID] = doc.Rel
			cs.Provides = append(cs.Provides, c)
		}
		cs.Consumes = append(cs.Consumes, parseConsumes(doc, &cs)...)
	}
	sort.SliceStable(cs.Provides, func(i, j int) bool { return cs.Provides[i].ID < cs.Provides[j].ID })
	sort.SliceStable(cs.Consumes, func(i, j int) bool {
		if cs.Consumes[i].Ref != cs.Consumes[j].Ref {
			return cs.Consumes[i].Ref < cs.Consumes[j].Ref
		}
		return cs.Consumes[i].Doc < cs.Consumes[j].Doc
	})
	return cs
}

func (cs *contractSet) problem(where, format string, args ...any) {
	cs.problems = append(cs.problems, Finding{"", "contract_schema", where, fmt.Sprintf(format, args...)})
}

// entries returns the list under key, or reports why it is not a list.
func entries(doc *Document, key string, cs *contractSet) []any {
	v, ok := doc.Meta[key]
	if !ok || v == nil {
		return nil
	}
	list, ok := v.([]any)
	if !ok {
		cs.problem(doc.Rel, "%s must be a list of entries", key)
		return nil
	}
	return list
}

// entryMap returns entry i as a mapping with only allowed keys, or reports it.
func entryMap(doc *Document, key string, i int, raw any, allowed []string, cs *contractSet) (map[string]any, bool) {
	m, ok := raw.(map[string]any)
	if !ok {
		cs.problem(doc.Rel, "%s entry %d must be a mapping (e.g. \"- %s: ...\")", key, i+1, allowed[0])
		return nil, false
	}
	var unknown []string
	for k := range m {
		if !contains(allowed, k) {
			unknown = append(unknown, k)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		cs.problem(doc.Rel, "%s entry %d has unknown key(s) %s (allowed: %s)",
			key, i+1, strings.Join(unknown, ", "), strings.Join(allowed, ", "))
		return nil, false
	}
	return m, true
}

// versionString returns a version value as written, rejecting values YAML
// parsed as a float: an unquoted "1.10" is read as 1.1, so comparing it would
// silently use the wrong number.
func versionString(doc *Document, what string, v any, cs *contractSet) (string, bool) {
	switch x := v.(type) {
	case nil:
		return "", true
	case string:
		return strings.TrimSpace(x), true
	case int64:
		return AnyToStr(x), true
	case float64:
		cs.problem(doc.Rel, "%s %s was read as a number; quote it (e.g. \"%s\") so it is not misread",
			what, AnyToStr(x), AnyToStr(x))
		return "", false
	default:
		cs.problem(doc.Rel, "%s must be a string", what)
		return "", false
	}
}

func stringField(doc *Document, what string, v any, cs *contractSet) (string, bool) {
	switch x := v.(type) {
	case nil:
		return "", true
	case string:
		return strings.TrimSpace(x), true
	default:
		cs.problem(doc.Rel, "%s must be a string", what)
		return "", false
	}
}

func parseProvides(doc *Document, cs *contractSet) []Contract {
	var out []Contract
	for i, raw := range entries(doc, "provides", cs) {
		m, ok := entryMap(doc, "provides", i, raw, providesKeys, cs)
		if !ok {
			continue
		}
		label := fmt.Sprintf("provides entry %d", i+1)
		id, okID := stringField(doc, label+" id", m["id"], cs)
		kind, okKind := stringField(doc, label+" kind", m["kind"], cs)
		spec, okSpec := stringField(doc, label+" spec", m["spec"], cs)
		version, okVer := versionString(doc, label+" version", m["version"], cs)
		if !okID || !okKind || !okSpec || !okVer {
			continue
		}
		switch {
		case id == "":
			cs.problem(doc.Rel, "%s is missing its id", label)
			continue
		case !contractIDRe.MatchString(id):
			cs.problem(doc.Rel, "contract id %s must be lowercase letters, digits and dashes, starting with a letter or digit", pyRepr(id))
			continue
		}
		if version != "" {
			if _, ok := parseSemver(version); !ok {
				cs.problem(doc.Rel, "contract %s has invalid version %s (expected semver, e.g. 2.1.0)", pyRepr(id), pyRepr(version))
				continue
			}
		}
		out = append(out, Contract{ID: id, Kind: kind, Version: version, Spec: spec, Doc: doc.Rel})
	}
	return out
}

func parseConsumes(doc *Document, cs *contractSet) []Dependency {
	var out []Dependency
	for i, raw := range entries(doc, "consumes", cs) {
		m, ok := entryMap(doc, "consumes", i, raw, consumesKeys, cs)
		if !ok {
			continue
		}
		label := fmt.Sprintf("consumes entry %d", i+1)
		ref, okRef := stringField(doc, label+" ref", m["ref"], cs)
		version, okVer := versionString(doc, label+" version", m["version"], cs)
		if !okRef || !okVer {
			continue
		}
		if ref == "" {
			cs.problem(doc.Rel, "%s is missing its ref", label)
			continue
		}
		if !validRef(ref) {
			cs.problem(doc.Rel, "ref %s must be a contract id, or owner/repo#id for another repository", pyRepr(ref))
			continue
		}
		if version != "" {
			if _, err := parseConstraint(version); err != nil {
				cs.problem(doc.Rel, "ref %s: %v", pyRepr(ref), err)
				continue
			}
		}
		out = append(out, Dependency{Ref: ref, Version: version, Doc: doc.Rel})
	}
	return out
}

func validRef(ref string) bool {
	repo, id, cross := strings.Cut(ref, "#")
	if !cross {
		return contractIDRe.MatchString(ref)
	}
	return repoNameRe.MatchString(repo) && contractIDRe.MatchString(id)
}

// specPaths returns the `spec` paths a document's provides entries declare.
// Specs count as related code (ADR 0013 §1): they must exist, and the doc is
// stale when its spec changes after it.
func (d *Document) specPaths() []string {
	list, _ := d.Meta["provides"].([]any)
	var out []string
	for _, raw := range list {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if s, ok := m["spec"].(string); ok && strings.TrimSpace(s) != "" {
			out = append(out, strings.TrimSpace(s))
		}
	}
	return out
}

// checkContracts validates contract edges within the repository. Cross-repo
// refs are recorded in the manifest but not resolved (ADR 0013 §5).
func checkContracts(model *DocsModel, cfg map[string]any) []Finding {
	sev := "warn"
	if checkBool(cfg, "contracts_is_error", true) {
		sev = "error"
	}
	cs := collectContracts(model)
	var findings []Finding
	for _, p := range cs.problems {
		p.Severity = sev
		findings = append(findings, p)
	}

	provided := map[string]Contract{}
	for _, c := range cs.Provides {
		provided[c.ID] = c
	}
	for _, d := range cs.Consumes {
		if !d.local() {
			continue
		}
		c, ok := provided[d.Ref]
		if !ok {
			findings = append(findings, Finding{sev, "contract_unresolved", d.Doc,
				fmt.Sprintf("consumes %s, but no doc in this repository provides it", pyRepr(d.Ref))})
			continue
		}
		if d.Version == "" {
			continue
		}
		if c.Version == "" {
			findings = append(findings, Finding{sev, "contract_version", d.Doc,
				fmt.Sprintf("consumes %s %s, but %s declares no version to check against",
					pyRepr(d.Ref), pyRepr(d.Version), c.Doc)})
			continue
		}
		preds, _ := parseConstraint(d.Version) // validated in parseConsumes
		ver, _ := parseSemver(c.Version)       // validated in parseProvides
		if !satisfies(ver, preds) {
			findings = append(findings, Finding{sev, "contract_version", d.Doc,
				fmt.Sprintf("consumes %s %s, but %s provides version %s",
					pyRepr(d.Ref), pyRepr(d.Version), c.Doc, c.Version)})
		}
	}
	return findings
}

// manifest is the generated .maat/manifest.json document.
type manifest struct {
	Schema   int          `json:"schema"`
	Repo     *string      `json:"repo"`
	Provides []Contract   `json:"provides"`
	Consumes []Dependency `json:"consumes"`
}

// manifestJSON renders the manifest: stable, sorted and indented so it diffs
// cleanly. Only valid entries are included; invalid ones are check findings.
func manifestJSON(model *DocsModel, cfg map[string]any) string {
	cs := collectContracts(model)
	m := manifest{Schema: manifestSchema, Provides: cs.Provides, Consumes: cs.Consumes}
	if m.Provides == nil {
		m.Provides = []Contract{}
	}
	if m.Consumes == nil {
		m.Consumes = []Dependency{}
	}
	if repo := configRepo(cfg); repo != "" {
		m.Repo = &repo
	}
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil { // cannot happen for these plain types
		panic(err)
	}
	return string(out) + "\n"
}

// configRepo returns the configured repository identity, or "" when unset.
func configRepo(cfg map[string]any) string {
	v, ok := cfg["repo"]
	if !ok || v == nil {
		return ""
	}
	return strings.TrimSpace(AnyToStr(v))
}

// contractsSection renders the llms.txt "Contracts" section, or "" when the
// repository declares no contract edges.
func contractsSection(model *DocsModel) string {
	cs := collectContracts(model)
	if len(cs.Provides) == 0 && len(cs.Consumes) == 0 {
		return ""
	}
	titles := map[string]string{}
	for _, doc := range model.Documents {
		titles[doc.Rel] = doc.Title()
	}
	lines := []string{"## Contracts"}
	for _, c := range cs.Provides {
		var details []string
		for _, s := range []string{c.Kind, c.Version} {
			if s != "" {
				details = append(details, s)
			}
		}
		note := ""
		if len(details) > 0 {
			note = " (" + strings.Join(details, ", ") + ")"
		}
		lines = append(lines, fmt.Sprintf("- Provides `%s`%s: [%s](%s)", c.ID, note, titles[c.Doc], c.Doc))
	}
	for _, d := range cs.Consumes {
		note := ""
		if d.Version != "" {
			note = " (" + d.Version + ")"
		}
		lines = append(lines, fmt.Sprintf("- Consumes `%s`%s: [%s](%s)", d.Ref, note, titles[d.Doc], d.Doc))
	}
	return strings.Join(lines, "\n") + "\n"
}
