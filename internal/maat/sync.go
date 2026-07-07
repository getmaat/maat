package maat

import (
	"os"
	"path/filepath"
	"strings"
)

// projectMeta returns a best-effort project name and summary for indexes.
func projectMeta(root string, cfg map[string]any) (name, summary string) {
	if v, ok := cfg["project_name"]; ok && truthy(v) {
		name = AnyToStr(v)
	} else {
		abs, err := filepath.Abs(root)
		if err != nil {
			abs = root
		}
		name = filepath.Base(abs)
	}
	if v, ok := cfg["project_summary"]; ok {
		summary = AnyToStr(v)
	}
	return name, summary
}

// adapterCtx computes the relative paths from an adapter file's location back
// to the instruction file and docs dir. depth = number of '/' in the adapter's
// relative path.
func adapterCtx(rel, docsDir, instructions, label string) adapterContext {
	depth := strings.Count(rel, "/")
	up := strings.Repeat("../", depth)
	return adapterContext{
		label:           label,
		docsDir:         docsDir,
		instructions:    instructions,
		instructionsRel: up + instructions,
		docsRel:         up + docsDir + "/",
		llmsRel:         up + docsDir + "/llms.txt",
	}
}

// orderedArtifact is one generated file: its repo-relative path and expected
// content. A slice (not a map) preserves deterministic emission order, matching
// the Python dict's insertion order.
type orderedArtifact struct {
	rel     string
	content string
}

// expectedArtifacts computes the full ordered list of generated files. Both
// sync (which writes them) and check (which compares) consume this, so
// "sync then check" is guaranteed to pass.
func expectedArtifacts(model *DocsModel, cfg map[string]any, root string) []orderedArtifact {
	var artifacts []orderedArtifact
	docsDir := AnyToStr(cfg["docs_dir"])
	instructions := AnyToStr(cfg["instructions_file"])
	name, summary := projectMeta(root, cfg)

	// 1. llms.txt — fully generated, at docs/llms.txt.
	artifacts = append(artifacts, orderedArtifact{docsDir + "/llms.txt", llmsTxt(model, name, summary)})

	// 2. docs/index.md — managed navigation region inside a hand-written file.
	indexRel := docsDir + "/index.md"
	existingIndex := readFileOrEmpty(filepath.Join(root, indexRel))
	artifacts = append(artifacts, orderedArtifact{indexRel, splice(existingIndex, indexNav(model))})

	// 3. Agent adapter files — managed pointer/mdc content.
	for _, target := range adaptersFor(cfg) {
		ctx := adapterCtx(target.path, docsDir, instructions, target.label)
		body := adapterContent(target.kind, ctx)
		if target.kind == "mdc" {
			artifacts = append(artifacts, orderedArtifact{target.path, body})
		} else {
			existing := readFileOrEmpty(filepath.Join(root, target.path))
			artifacts = append(artifacts, orderedArtifact{target.path, splice(existing, body)})
		}
	}

	// 4. Agent skills (ADR 0007) — canonical copies under .maat/skills/ and
	// copies fanned out to vendor-native skill directories of the enabled
	// adapters. The skill files themselves are only emitted when skills exist.
	for _, def := range skillDefs {
		body := skillContent(def, docsDir, instructions)
		artifacts = append(artifacts, orderedArtifact{skillsRoot + "/" + def.name + "/SKILL.md", body})
		for _, target := range adaptersFor(cfg) {
			if target.skillsDir == "" {
				continue
			}
			artifacts = append(artifacts, orderedArtifact{target.skillsDir + "/maat-" + def.name + "/SKILL.md", body})
		}
	}

	// 5. Maintenance-contract block spliced into the instruction file (ADR 0009):
	// the update protocol, the front-matter schema, and the skills discovery
	// list. This is emitted unconditionally — even with zero skills — so a
	// brownfield instruction file that `init` skipped still gains Ma'at's
	// contract non-destructively, and it self-heals on every sync.
	existing := readFileOrEmpty(filepath.Join(root, instructions))
	artifacts = append(artifacts, orderedArtifact{instructions, splice(existing, contractBlock(skillDefs, docsDir))})
	return artifacts
}

// writeArtifacts writes every artifact to disk, creating parent dirs. Returns
// the list of changed paths (unchanged files are left untouched).
func writeArtifacts(artifacts []orderedArtifact, root string) ([]string, error) {
	var changed []string
	for _, art := range artifacts {
		content := art.content
		if !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		path := filepath.Join(root, art.rel)
		if readFileOrEmpty(path) == content {
			continue
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
		changed = append(changed, art.rel)
	}
	return changed, nil
}

// RunSync scans the docs tree, computes expected artifacts, and writes them.
// Returns the changed paths.
func RunSync(root string, cfg map[string]any) ([]string, error) {
	model, err := ScanModel(root, AnyToStr(cfg["docs_dir"]))
	if err != nil {
		return nil, err
	}
	return writeArtifacts(expectedArtifacts(model, cfg, root), root)
}
