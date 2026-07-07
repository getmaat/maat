package maat

import (
	"strings"
)

// Managed-region markers. Generated content lives between these so a file can
// mix generated and hand-written sections.
const (
	beginMarker = "<!-- maat:begin (generated — edit the source docs, not this block) -->"
	endMarker   = "<!-- maat:end -->"
)

// skillsRoot is the canonical, agent-agnostic location of Ma'at-managed agent
// skills (ADR 0007). Vendor-native skill directories receive copies.
const skillsRoot = ".maat/skills"

// splice inserts or replaces the managed region inside existing. If no markers
// are present, the block is appended. Text outside the markers is preserved.
func splice(existing, generated string) string {
	block := beginMarker + "\n" + strings.TrimRight(generated, "\n") + "\n" + endMarker
	if strings.Contains(existing, beginMarker) && strings.Contains(existing, endMarker) {
		pre := strings.Split(existing, beginMarker)[0]
		post := strings.SplitN(existing, endMarker, 2)[1]
		return pre + block + post
	}
	if strings.TrimSpace(existing) != "" {
		return strings.TrimRight(existing, "\n") + "\n\n" + block + "\n"
	}
	return block + "\n"
}

// llmsTxt renders an llms.txt index of the docs tree (per llmstxt.org): an H1
// name, a ">" summary, then H2 sections each a bullet list of links.
func llmsTxt(model *DocsModel, projectName, projectSummary string) string {
	var lines []string
	lines = append(lines, "# "+projectName, "")
	if projectSummary != "" {
		lines = append(lines, "> "+projectSummary, "")
	}
	lines = append(lines,
		"This file indexes the project's documentation for AI agents and "+
			"tools. Each link points to a Markdown document; agents should read "+
			"the ones relevant to the task before making changes.",
		"")

	buckets := model.bySection()
	for _, section := range model.orderedSections() {
		docs := buckets[section]
		if len(docs) == 0 {
			continue
		}
		lines = append(lines, "## "+sectionTitle(section))
		for _, doc := range docs {
			suffix := ""
			if note := doc.Summary(); note != "" {
				suffix = ": " + note
			}
			lines = append(lines, "- ["+doc.Title()+"]("+doc.Rel+")"+suffix)
		}
		lines = append(lines, "")
	}

	// Root-level docs (index.md etc.) get their own section.
	if rootDocs := buckets["_root"]; len(rootDocs) > 0 {
		lines = append(lines, "## Entry points")
		for _, doc := range rootDocs {
			suffix := ""
			if note := doc.Summary(); note != "" {
				suffix = ": " + note
			}
			lines = append(lines, "- ["+doc.Title()+"]("+doc.Rel+")"+suffix)
		}
		lines = append(lines, "")
	}

	return strings.TrimRight(strings.Join(lines, "\n"), "\n") + "\n"
}

// indexNav renders the generated navigation block for docs/index.md.
func indexNav(model *DocsModel) string {
	var lines []string
	buckets := model.bySection()
	for _, section := range model.orderedSections() {
		docs := buckets[section]
		if len(docs) == 0 {
			continue
		}
		lines = append(lines, "### "+sectionTitle(section), "")
		for _, doc := range docs {
			// index.md lives in docs/, so links are relative to it.
			rel := doc.Rel[len(model.DocsDir)+1:]
			suffix := ""
			if note := doc.Summary(); note != "" {
				suffix = " — " + note
			}
			status := ""
			if doc.Status() != "current" {
				status = " _(" + doc.Status() + ")_"
			}
			lines = append(lines, "- ["+doc.Title()+"]("+rel+")"+status+suffix)
		}
		lines = append(lines, "")
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}

// adapterContext holds the substitution values for an adapter template.
type adapterContext struct {
	label           string
	docsDir         string
	instructions    string
	instructionsRel string
	docsRel         string
	llmsRel         string
}

// adapterContent renders an adapter file body for the given kind. The pointer
// and mdc templates are embedded (see scaffold.go).
func adapterContent(kind string, ctx adapterContext) string {
	var tmpl string
	if kind == "mdc" {
		tmpl = adapterMDCTemplate
	} else {
		tmpl = adapterPointerTemplate
	}
	repl := strings.NewReplacer(
		"{label}", ctx.label,
		"{docs_dir}", ctx.docsDir,
		"{instructions}", ctx.instructions,
		"{instructions_rel}", ctx.instructionsRel,
		"{docs_rel}", ctx.docsRel,
		"{llms_rel}", ctx.llmsRel,
	)
	return repl.Replace(tmpl)
}

// skillVersionStamp is the version written into generated skills (ADR 0007) so
// drift is attributable to a binary version. Development builds stamp "dev"
// rather than their per-commit pseudo-version, so contributors regenerating
// with source builds do not thrash the drift check.
func skillVersionStamp() string {
	v := Version()
	if isDevBuild(v) {
		return "dev"
	}
	return v
}

// skillContent renders one managed skill body from its embedded template
// (ADR 0007). Skills are whole-file owned by Ma'at: sync regenerates them,
// check flags drift, hand-edits are overwritten.
func skillContent(def skillDef, docsDir, instructions string) string {
	repl := strings.NewReplacer(
		"{maat_version}", skillVersionStamp(),
		"{docs_dir}", docsDir,
		"{instructions}", instructions,
	)
	return repl.Replace(tmpl(def.tmpl))
}

// contractBlock renders the managed "maintenance contract" section spliced into
// the instruction file. It carries Ma'at's framework invariants — the
// documentation update protocol, the front-matter schema, and the agent-skills
// discovery list (ADR 0007) — as generated content, so that even a brownfield
// instruction file that `init` preserved untouched gains the contract
// non-destructively, and it self-heals on every `sync` (ADR 0009). Only
// genuinely project-specific prose (the overview, the docs map) is left to
// humans and the retrospect skill (ADR 0008). The paths are parameterized on
// docsDir so a repo with a non-default docs directory gets correct links.
//
// The block is the agent-agnostic mechanism: any agent that honors the
// instruction file can follow these relative links, so no native skills or
// protocol feature is required of the harness.
func contractBlock(defs []skillDef, docsDir string) string {
	var lines []string
	lines = append(lines,
		// Heading kept verbatim: the generated adapter pointers reference the
		// instruction file "under \"Documentation update protocol\"".
		"## Documentation update protocol",
		"",
		"**A change is not complete until its documentation is updated in the same",
		"change.** Treat docs edits as part of the diff, never a follow-up.",
		"",
		"When you modify code, update docs as follows:",
		"",
		"| If you… | Then update… |",
		"|---|---|",
		"| Change how a module works or how modules relate | the module's page in `"+docsDir+"/architecture/` |",
		"| Make a non-obvious, hard-to-reverse choice | add a new ADR in `"+docsDir+"/decisions/` (copy `_template.md`) |",
		"| Change build/test/deploy/run steps | the relevant `"+docsDir+"/guides/` page |",
		"| Add/rename/remove a CLI flag, config key, or front-matter field | `"+docsDir+"/reference/` |",
		"| Add or move a source file a doc's `related_code` points at | that doc's `related_code` front-matter |",
		"",
		"Then regenerate derived indexes and adapter files, and validate before",
		"committing:",
		"",
		"```bash",
		"maat sync      # regenerate llms.txt, index nav, adapters, and this block",
		"maat check     # fails on stale/broken/missing/drifted docs",
		"```",
		"",
		"### Front-matter every doc carries",
		"",
		"Each Markdown file in `"+docsDir+"/` begins with a front-matter block. The",
		"`related_code` list is what lets tooling detect when code drifts from docs:",
		"",
		"```markdown",
		"---",
		"title: Human-readable title",
		"status: current            # current | draft | deprecated",
		"summary: One-line description used in indexes.",
		"related_code:              # source paths this doc describes (optional)",
		"  - src/module/thing.ext",
		"---",
		"```",
		"",
		"## Skills (reusable procedures)",
		"",
		"Ma'at ships step-by-step procedures for recurring documentation tasks",
		"under `"+skillsRoot+"/`. When a task matches one, read the skill file",
		"and follow it.",
		"")
	for _, def := range defs {
		lines = append(lines, "- [`"+def.name+"`]("+skillsRoot+"/"+def.name+"/SKILL.md) — "+def.desc)
	}
	lines = append(lines, "",
		"These files are generated — `maat sync` regenerates them, and hand-edits",
		"are overwritten. Team-authored skills may live alongside them and are",
		"never touched.")
	return strings.Join(lines, "\n")
}
