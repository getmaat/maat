package maat

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// adapterTarget describes one agent adapter file: where it lives, how it is
// rendered ("pointer" = markdown stub, "mdc" = Cursor rule with front-matter),
// and — for agents with native skill loading — the vendor directory that
// Ma'at-managed skills are additionally copied into (ADR 0007).
type adapterTarget struct {
	path      string
	kind      string
	label     string
	skillsDir string // vendor-native skills dir, "" = no native skill support
}

// adapterTargets fans the single source of truth (AGENTS.md) out to agents that
// insist on their own filename. Mirrors ADAPTER_TARGETS in the Python config.
var adapterTargets = map[string]adapterTarget{
	"claude":   {"CLAUDE.md", "pointer", "Claude Code", ".claude/skills"},
	"hermes":   {".hermes.md", "pointer", "Hermes", ""},
	"copilot":  {".github/copilot-instructions.md", "pointer", "GitHub Copilot", ""},
	"cursor":   {".cursor/rules/maat.mdc", "mdc", "Cursor", ""},
	"windsurf": {".windsurf/rules/maat.md", "pointer", "Windsurf", ""},
	"gemini":   {"GEMINI.md", "pointer", "Gemini CLI", ""},
}

const configFilename = ".maat.yml"

// defaultConfig returns a fresh copy of the default configuration. Every knob
// has a default so a repo can adopt Ma'at with an empty or absent config
// file and still get sensible behaviour.
func defaultConfig() map[string]any {
	return map[string]any{
		"docs_dir":             "docs",
		"instructions_file":    "AGENTS.md",
		"adapters":             []any{"claude", "hermes", "copilot", "cursor", "windsurf", "gemini"},
		"required_frontmatter": []any{"title", "status"},
		"statuses":             []any{"current", "draft", "deprecated"},
		"check": map[string]any{
			"orphaned_code_is_error": true,
			"broken_links_is_error":  true,
			"drift_is_error":         true,
			"staleness":              "warn",
			"ignore_code_paths":      []any{},
		},
	}
}

// mergeConfig deep-merges override onto base (recursing into nested mappings),
// matching the Python _merge: list values replace, map values merge.
func mergeConfig(base, override map[string]any) map[string]any {
	out := make(map[string]any, len(base))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range override {
		if vm, ok := v.(map[string]any); ok {
			if bm, ok := out[k].(map[string]any); ok {
				out[k] = mergeConfig(bm, vm)
				continue
			}
		}
		out[k] = v
	}
	return out
}

// LoadConfig loads merged config from <root>/.maat.yml, or defaults when the
// file is absent or empty.
func LoadConfig(root string) (map[string]any, error) {
	path := filepath.Join(root, configFilename)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultConfig(), nil
		}
		return nil, err
	}
	raw, err := yamlParse(string(data))
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return defaultConfig(), nil
	}
	rawMap, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be a mapping at the top level", configFilename)
	}
	merged := mergeConfig(defaultConfig(), rawMap)
	if err := validateConfig(merged); err != nil {
		return nil, err
	}
	return merged, nil
}

func validateConfig(cfg map[string]any) error {
	for _, name := range toStringList(cfg["adapters"]) {
		if _, ok := adapterTargets[name]; !ok {
			known := make([]string, 0, len(adapterTargets))
			for k := range adapterTargets {
				known = append(known, k)
			}
			sort.Strings(known)
			return fmt.Errorf("unknown adapter %s (known: %s)", pyRepr(name), strings.Join(known, ", "))
		}
	}
	staleness := checkStr(cfg, "staleness", "")
	if staleness != "off" && staleness != "warn" && staleness != "error" {
		return fmt.Errorf("check.staleness must be off|warn|error, got %s", pyRepr(staleness))
	}
	// maat_version is optional; when present it must be a well-formed
	// constraint so a typo fails fast for every build (including dev builds
	// that are otherwise exempt from enforcement — see ADR 0006).
	if v, ok := cfg["maat_version"]; ok && truthy(v) {
		if _, err := parseConstraint(AnyToStr(v)); err != nil {
			return fmt.Errorf("invalid maat_version constraint: %w", err)
		}
	}
	return nil
}

// enforceVersion applies the optional `maat_version` constraint to the running
// binary (ADR 0006). It returns a non-nil error when a real release binary
// fails to satisfy the repo's declared constraint; the caller turns that into
// an exit-2 configuration error with an upgrade hint. Development builds
// (source, `go run`, VCS pseudo-versions) are exempt so contributors are never
// blocked — the constraint governs released binaries, which are what teams and
// CI pin. The constraint syntax was already validated in validateConfig, so a
// parse error here is treated as unenforced rather than re-reported.
func enforceVersion(cfg map[string]any) error {
	v, ok := cfg["maat_version"]
	if !ok || !truthy(v) {
		return nil
	}
	constraint := AnyToStr(v)
	enforced, satisfied, err := checkVersionConstraint(Version(), constraint)
	if err != nil || !enforced || satisfied {
		return nil
	}
	return fmt.Errorf(
		"this repository requires maat %s but you are running %s.\n"+
			"Upgrade: `brew upgrade maat`, re-run the install script, or "+
			"`go install github.com/getmaat/maat@latest`.\n"+
			"To change the requirement, edit `maat_version` in %s.",
		constraint, Version(), configFilename)
}

// resolvedAdapter is an enabled adapter with its target details.
type resolvedAdapter struct {
	name      string
	path      string
	kind      string
	label     string
	skillsDir string
}

// adaptersFor returns resolved adapter descriptors for the enabled adapters, in
// config order.
func adaptersFor(cfg map[string]any) []resolvedAdapter {
	var result []resolvedAdapter
	for _, name := range toStringList(cfg["adapters"]) {
		t := adapterTargets[name]
		result = append(result, resolvedAdapter{name: name, path: t.path, kind: t.kind, label: t.label, skillsDir: t.skillsDir})
	}
	return result
}

// checkBool reads a boolean from the nested check map, falling back to def.
func checkBool(cfg map[string]any, key string, def bool) bool {
	check, _ := cfg["check"].(map[string]any)
	if check == nil {
		return def
	}
	v, ok := check[key]
	if !ok {
		return def
	}
	return truthy(v)
}

// checkStr reads a string from the nested check map, falling back to def.
func checkStr(cfg map[string]any, key, def string) string {
	check, _ := cfg["check"].(map[string]any)
	if check == nil {
		return def
	}
	v, ok := check[key]
	if !ok {
		return def
	}
	if s, ok := v.(string); ok {
		return s
	}
	return def
}

// checkVal returns a raw value from the nested check map (nil if absent).
func checkVal(cfg map[string]any, key string) any {
	check, _ := cfg["check"].(map[string]any)
	if check == nil {
		return nil
	}
	return check[key]
}
