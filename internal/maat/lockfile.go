package maat

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Cross-repo contract resolution (ADR 0014). `maat contracts update` fetches
// the manifests of consumed repositories into a committed lockfile; `check`
// validates cross-repo refs against that lockfile, offline. This file holds
// the configuration, the lockfile format and the offline check. Fetching
// lives elsewhere and is the only networked code path.

// lockPath is the committed snapshot of consumed contracts.
const lockPath = ".maat/contracts.lock.json"

// lockSchema versions the lockfile format.
const lockSchema = 1

// contractSource is one configured `contracts.sources` entry: where to find
// another repository's manifest. At most one of URL and Path is set; with
// neither, the manifest is fetched from GitHub.
type contractSource struct {
	Ref  string
	URL  string
	Path string
}

// lockedContract is a provider's contract as recorded in the lockfile.
type lockedContract struct {
	ID      string `json:"id"`
	Kind    string `json:"kind,omitempty"`
	Version string `json:"version,omitempty"`
	Doc     string `json:"doc,omitempty"`
}

// lockedSource is one repository's snapshot. Settings mirror the source
// configuration it was fetched with, so a config change is detectable.
// Provides holds the consumed contracts the provider has; Missing the consumed
// ids it does not provide. An id in neither was not requested when the
// lockfile was written (the lockfile is out of date).
type lockedSource struct {
	Ref            string           `json:"ref,omitempty"`
	URL            string           `json:"url,omitempty"`
	Path           string           `json:"path,omitempty"`
	Commit         string           `json:"commit,omitempty"`
	ManifestSchema int              `json:"manifest_schema"`
	Provides       []lockedContract `json:"provides"`
	Missing        []string         `json:"missing,omitempty"`
}

// contractsLock is the .maat/contracts.lock.json document. Map keys are
// emitted sorted by encoding/json, so the file is deterministic.
type contractsLock struct {
	Schema  int                     `json:"schema"`
	Sources map[string]lockedSource `json:"sources"`
}

var sourceKeys = []string{"ref", "url", "path"}

// validateContractSources checks `contracts.sources` in .maat.yml. It is part
// of config validation, so a malformed source is a configuration error
// (exit 2) rather than a finding.
func validateContractSources(cfg map[string]any) error {
	raw, ok := cfg["contracts"]
	if !ok || raw == nil {
		return nil
	}
	contracts, ok := raw.(map[string]any)
	if !ok {
		return fmt.Errorf("contracts must be a mapping")
	}
	for k := range contracts {
		if k != "sources" {
			return fmt.Errorf("contracts.%s is not a known key (allowed: sources)", k)
		}
	}
	rawSources, ok := contracts["sources"]
	if !ok || rawSources == nil {
		return nil
	}
	sources, ok := rawSources.(map[string]any)
	if !ok {
		return fmt.Errorf("contracts.sources must be a mapping of owner/name to source settings")
	}
	for repo, v := range sources {
		where := "contracts.sources." + repo
		if !repoNameRe.MatchString(repo) {
			return fmt.Errorf("%s: source keys must be \"owner/name\"", where)
		}
		if v == nil {
			continue // all defaults
		}
		if s, isStr := v.(string); isStr && strings.TrimSpace(s) == "{}" {
			return fmt.Errorf("%s: write an empty value (\"%s:\") instead of {} for default settings", where, repo)
		}
		m, isMap := v.(map[string]any)
		if !isMap {
			return fmt.Errorf("%s must be empty or a mapping of %s", where, strings.Join(sourceKeys, ", "))
		}
		for k, val := range m {
			if !contains(sourceKeys, k) {
				return fmt.Errorf("%s.%s is not a known key (allowed: %s)", where, k, strings.Join(sourceKeys, ", "))
			}
			switch val.(type) {
			case string, int64:
			case float64:
				return fmt.Errorf("%s.%s was read as a number; quote it", where, k)
			default:
				return fmt.Errorf("%s.%s must be a string", where, k)
			}
		}
		src := sourceFromMap(m)
		if src.URL != "" && src.Path != "" {
			return fmt.Errorf("%s: set url or path, not both", where)
		}
		if src.URL != "" && !strings.HasPrefix(src.URL, "https://") {
			// Tokens may be sent to this URL; never over plain HTTP.
			return fmt.Errorf("%s.url must start with https://", where)
		}
	}
	return nil
}

func sourceFromMap(m map[string]any) contractSource {
	get := func(k string) string {
		if v, ok := m[k]; ok && v != nil {
			return strings.TrimSpace(AnyToStr(v))
		}
		return ""
	}
	return contractSource{Ref: get("ref"), URL: get("url"), Path: get("path")}
}

// contractSources returns the configured sources. The config has already been
// validated, so malformed entries cannot occur here.
func contractSources(cfg map[string]any) map[string]contractSource {
	out := map[string]contractSource{}
	contracts, _ := cfg["contracts"].(map[string]any)
	sources, _ := contracts["sources"].(map[string]any)
	for repo, v := range sources {
		m, _ := v.(map[string]any)
		out[repo] = sourceFromMap(m)
	}
	return out
}

// matches reports whether a locked snapshot was taken with these settings.
func (s contractSource) matches(l lockedSource) bool {
	return s.Ref == l.Ref && s.URL == l.URL && s.Path == l.Path
}

// readLock loads the lockfile. A missing file is not an error.
func readLock(root string) (lock *contractsLock, exists bool, err error) {
	data, err := os.ReadFile(filepath.Join(root, lockPath))
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, true, err
	}
	var l contractsLock
	if err := json.Unmarshal(data, &l); err != nil {
		return nil, true, fmt.Errorf("not valid JSON: %w", err)
	}
	if l.Schema != lockSchema {
		return nil, true, fmt.Errorf("lockfile schema %d is not supported by this maat (expects %d)", l.Schema, lockSchema)
	}
	return &l, true, nil
}

// splitRef splits "owner/repo#id" into its parts; ok is false for a local ref.
func splitRef(ref string) (repo, id string, ok bool) {
	return strings.Cut(ref, "#")
}

// checkCrossRepo validates cross-repo consumes refs against the lockfile
// (ADR 0014 §3). It never touches the network. Refs to the repository itself
// (its configured `repo`) are resolved locally by checkContracts instead.
func checkCrossRepo(model *DocsModel, cfg map[string]any, deps []Dependency) []Finding {
	contractSev := "warn"
	if checkBool(cfg, "contracts_is_error", true) {
		contractSev = "error"
	}
	driftSev := "warn"
	if checkBool(cfg, "drift_is_error", true) {
		driftSev = "error"
	}

	sources := contractSources(cfg)
	byRepo := map[string][]Dependency{}
	var findings []Finding
	for _, d := range deps {
		repo, _, _ := splitRef(d.Ref)
		if _, ok := sources[repo]; !ok {
			findings = append(findings, Finding{contractSev, "contract_schema", d.Doc,
				fmt.Sprintf("consumes %s, but %s has no contracts.sources entry for %s",
					pyRepr(d.Ref), configFilename, repo)})
			continue
		}
		byRepo[repo] = append(byRepo[repo], d)
	}

	lock, exists, err := readLock(model.Root)
	drift := func(msg string) {
		findings = append(findings, Finding{driftSev, "drift", lockPath, msg})
	}
	switch {
	case err != nil:
		drift(fmt.Sprintf("unreadable (%v) — run `maat contracts update`", err))
		return findings
	case len(byRepo) == 0:
		if exists {
			drift("no cross-repo contracts are consumed any more — run `maat contracts update` to remove it")
		}
		return findings
	case !exists:
		drift("missing — run `maat contracts update`")
		return findings
	}

	repos := make([]string, 0, len(byRepo))
	for repo := range byRepo {
		repos = append(repos, repo)
	}
	sort.Strings(repos)
	var stale []string
	for repo := range lock.Sources {
		if _, ok := byRepo[repo]; !ok {
			stale = append(stale, repo)
		}
	}
	sort.Strings(stale)
	for _, repo := range stale {
		drift(fmt.Sprintf("lists %s, which is no longer consumed — run `maat contracts update`", repo))
	}

	for _, repo := range repos {
		locked, ok := lock.Sources[repo]
		if !ok {
			drift(fmt.Sprintf("has no entry for %s — run `maat contracts update`", repo))
			continue
		}
		if !sources[repo].matches(locked) {
			drift(fmt.Sprintf("entry for %s was fetched with different source settings — run `maat contracts update`", repo))
			continue
		}
		provided := map[string]lockedContract{}
		for _, c := range locked.Provides {
			provided[c.ID] = c
		}
		at := repo
		if locked.Commit != "" {
			at = fmt.Sprintf("%s@%s", repo, shortCommit(locked.Commit))
		}
		for _, d := range byRepo[repo] {
			_, id, _ := splitRef(d.Ref)
			c, ok := provided[id]
			switch {
			case ok:
				if f, bad := versionFinding(d, c.Version, at, contractSev); bad {
					findings = append(findings, f)
				}
			case contains(locked.Missing, id):
				findings = append(findings, Finding{contractSev, "contract_unresolved", d.Doc,
					fmt.Sprintf("consumes %s, but %s does not provide %s", pyRepr(d.Ref), at, pyRepr(id))})
			default:
				drift(fmt.Sprintf("has no entry for %s — run `maat contracts update`", d.Ref))
			}
		}
	}
	return findings
}

// versionFinding checks a consumer's constraint against a provided version.
// provider names where the version comes from, for the message.
func versionFinding(d Dependency, version, provider, sev string) (Finding, bool) {
	if d.Version == "" {
		return Finding{}, false
	}
	if version == "" {
		return Finding{sev, "contract_version", d.Doc,
			fmt.Sprintf("consumes %s %s, but %s declares no version to check against",
				pyRepr(d.Ref), pyRepr(d.Version), provider)}, true
	}
	preds, err := parseConstraint(d.Version) // validated in parseConsumes
	if err != nil {
		return Finding{}, false
	}
	ver, ok := parseSemver(version)
	if !ok {
		return Finding{sev, "contract_version", d.Doc,
			fmt.Sprintf("consumes %s, but %s declares an invalid version %s",
				pyRepr(d.Ref), provider, pyRepr(version))}, true
	}
	if !satisfies(ver, preds) {
		return Finding{sev, "contract_version", d.Doc,
			fmt.Sprintf("consumes %s %s, but %s provides version %s",
				pyRepr(d.Ref), pyRepr(d.Version), provider, version)}, true
	}
	return Finding{}, false
}

func shortCommit(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}
