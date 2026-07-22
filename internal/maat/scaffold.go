package maat

import (
	"embed"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// templatesFS holds the scaffold content written by `maat init`. These are
// byte-for-byte the same templates the reference Python implementation ships,
// extracted from codedoc/scaffold.py at build time. The Ma'at repo's own
// docs are hand-written (richer than the scaffold) so the project dogfoods the
// framework; only init consumes these.
//
//go:embed templates/*
var templatesFS embed.FS

func tmpl(name string) string {
	data, err := templatesFS.ReadFile("templates/" + name)
	if err != nil {
		panic("missing embedded template: " + name)
	}
	return string(data)
}

// adapterPointerTemplate / adapterMDCTemplate are the two adapter renderings,
// consumed by generate.go.
var (
	adapterPointerTemplate = tmpl("adapter_pointer.txt")
	adapterMDCTemplate     = tmpl("adapter_mdc.txt")
)

// skillDef is one Ma'at-shipped agent skill (ADR 0007): a reusable procedure
// generated to .maat/skills/<name>/SKILL.md from an embedded template, listed
// in the instruction file's managed skills block, and fanned out to vendor
// skill directories. Like the adapters, skills are managed artifacts: sync
// regenerates them, check flags drift, and the content is coupled to the
// binary version via the embedded template.
type skillDef struct {
	name string // directory name under .maat/skills/
	tmpl string // embedded template filename
	desc string // one-line trigger description for the skills block
}

// skillDefs is the ordered set of skills Ma'at ships. Order is emission order.
var skillDefs = []skillDef{
	{
		name: "retrospect",
		tmpl: "skill_retrospect.md",
		desc: "Retrofit Ma'at documentation onto an existing repository: " +
			"inventory gaps, interview the developer, derive documentation " +
			"and retrospective ADRs.",
	},
}

// scaffoldFile pairs a repo-relative destination with its template name.
type scaffoldFile struct {
	rel  string
	name string
}

// scaffoldFiles is the ordered set of files init stamps out, mirroring
// init._FILES in the Python implementation (note the leading-underscore
// naming shared by both hand-copy templates: docs/decisions/_template.md and
// .maat/templates/_module.md — see ADR 0010).
var scaffoldFiles = []scaffoldFile{
	{"AGENTS.md", "AGENTS.md"},
	{"docs/index.md", "index.md"},
	{"docs/architecture/overview.md", "arch_overview.md"},
	{"docs/decisions/README.md", "decisions_README.md"},
	{"docs/decisions/_template.md", "adr_template.md"},
	{"docs/decisions/0001-record-architecture-decisions.md", "adr_0001.md"},
	{"docs/guides/development.md", "guide_development.md"},
	{"docs/guides/testing.md", "guide_testing.md"},
	{"docs/guides/deployment.md", "guide_deployment.md"},
	{"docs/guides/troubleshooting.md", "guide_troubleshooting.md"},
	{"docs/reference/configuration.md", "reference_config.md"},
	{"docs/reference/environment.md", "reference_env.md"},
	{"docs/meta/conventions.md", "meta_conventions.md"},
	{"docs/meta/glossary.md", "meta_glossary.md"},
	{"docs/meta/maintenance.md", "meta_maintenance.md"},
	{".maat/templates/_module.md", "module_template.md"},
	{".maat.yml", "config.yml"},
	{".github/workflows/maat.yml", "workflow.yml"},
}

// scaffoldVersionPin returns the value stamped into the scaffolded CI
// workflow's MAAT_VERSION. A real release binary pins its own exact version so
// generated CI is reproducible and matches the tool that wrote it; a
// development build leaves it empty, meaning "track the latest release" (we
// can't pin a dev build users can't install). See ADR 0006.
func scaffoldVersionPin() string {
	v := Version()
	if isDevBuild(v) {
		return ""
	}
	return v
}

// scaffoldActionRef returns the git ref used to pin the composite Action /
// reusable workflow in scaffolded CI (e.g. "v0.2.0"). Like scaffoldVersionPin
// it matches the scaffolding binary's own release for reproducibility. A
// development build can't know a real release tag, so it emits an obvious
// "vX.Y.Z" placeholder the user replaces. A moving major pointer (@v1) is
// intentionally not used until 1.0, so it can't collide with the semver-tag
// release trigger. See ADR 0006.
func scaffoldActionRef() string {
	v := Version()
	if isDevBuild(v) {
		return "vX.Y.Z"
	}
	return "v" + v
}

// adaptersBlock renders the config.yml `adapters:` key and its list, for
// however many agents were chosen (zero is valid: AGENTS.md-only, no
// per-agent adapter files at all).
func adaptersBlock(agents []string) string {
	if len(agents) == 0 {
		return "adapters: []"
	}
	var b strings.Builder
	b.WriteString("adapters:")
	for _, a := range agents {
		b.WriteString("\n  - " + a)
	}
	return b.String()
}

func fill(text string, subs map[string]string) string {
	// Longest keys first so {{SUMMARY_INLINE}} is not partially matched by
	// {{SUMMARY}}. Go maps are unordered, so sort explicitly.
	keys := make([]string, 0, len(subs))
	for k := range subs {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })
	for _, k := range keys {
		text = strings.ReplaceAll(text, "{{"+k+"}}", subs[k])
	}
	return text
}

// InitResult reports what init did.
type InitResult struct {
	Created   []string
	Skipped   []string
	Generated []string
}

// RunInit scaffolds Ma'at into root. Existing files are skipped unless force
// is set, so re-running init is safe. agents is the exact adapters: list to
// stamp into a freshly-written .maat.yml (see adaptersBlock) — callers
// resolve defaults/flags/wizard input before calling this. After stamping
// files it runs sync.
func RunInit(root, project, summary string, agents []string, force bool) (*InitResult, error) {
	summary = strings.TrimSpace(summary)
	if summary == "" {
		summary = "TODO: one-paragraph description of this project."
	}
	inline := strings.ReplaceAll(summary, "\n", " ")
	if len(inline) > 200 {
		inline = inline[:200]
	}
	subs := map[string]string{
		"PROJECT":         project,
		"SUMMARY":         summary,
		"SUMMARY_INLINE":  inline,
		"DATE":            time.Now().Format("2006-01-02"),
		"NAME":            "example",
		"PATH":            "src/example",
		"MAAT_VERSION":    scaffoldVersionPin(),
		"MAAT_ACTION_REF": scaffoldActionRef(),
		"ADAPTERS_BLOCK":  adaptersBlock(agents),
	}

	result := &InitResult{}
	for _, f := range scaffoldFiles {
		path := filepath.Join(root, f.rel)
		if pathExists(path) && !force {
			result.Skipped = append(result.Skipped, f.rel)
			continue
		}
		content := fill(tmpl(f.name), subs)
		if !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		dir := filepath.Dir(path)
		if dir != "" {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, err
			}
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return nil, err
		}
		result.Created = append(result.Created, f.rel)
	}

	cfg, err := LoadConfig(root)
	if err != nil {
		return nil, err
	}
	generated, err := RunSync(root, cfg)
	if err != nil {
		return nil, err
	}
	result.Generated = generated
	return result, nil
}
