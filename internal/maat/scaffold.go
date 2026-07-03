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

// scaffoldFile pairs a repo-relative destination with its template name.
type scaffoldFile struct {
	rel  string
	name string
}

// scaffoldFiles is the ordered set of files init stamps out, mirroring
// init._FILES in the Python implementation (note _template.md and the two
// templates/ copies).
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
	{"templates/adr.md", "adr_template.md"},
	{"templates/module.md", "module_template.md"},
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
// is set, so re-running init is safe. After stamping files it runs sync.
func RunInit(root, project, summary string, force bool) (*InitResult, error) {
	summary = strings.TrimSpace(summary)
	if summary == "" {
		summary = "TODO: one-paragraph description of this project."
	}
	inline := strings.ReplaceAll(summary, "\n", " ")
	if len(inline) > 200 {
		inline = inline[:200]
	}
	subs := map[string]string{
		"PROJECT":        project,
		"SUMMARY":        summary,
		"SUMMARY_INLINE": inline,
		"DATE":           time.Now().Format("2006-01-02"),
		"NAME":           "example",
		"PATH":           "src/example",
		"MAAT_VERSION":   scaffoldVersionPin(),
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
