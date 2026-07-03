package maat

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Finding is one validation result. Severity is "error" or "warn".
type Finding struct {
	Severity string
	Code     string
	Where    string
	Message  string
}

// String renders a finding the same way the Python __str__ does.
func (f Finding) String() string {
	icon := "⚠"
	if f.Severity == "error" {
		icon = "✖"
	}
	return fmt.Sprintf("%s [%s] %s: %s", icon, f.Code, f.Where, f.Message)
}

func isExternal(link string) bool {
	return strings.Contains(link, "://") ||
		strings.HasPrefix(link, "#") ||
		strings.HasPrefix(link, "mailto:") ||
		strings.HasPrefix(link, "//")
}

func isIgnored(path string, prefixes []string) bool {
	for _, p := range prefixes {
		if path == p || strings.HasPrefix(path, strings.TrimRight(p, "/")+"/") {
			return true
		}
	}
	return false
}

func checkFrontmatter(model *DocsModel, cfg map[string]any) []Finding {
	var findings []Finding
	required := toStringList(cfg["required_frontmatter"])
	statuses := toStringList(cfg["statuses"])
	for _, doc := range model.Documents {
		for _, key := range required {
			if v, ok := doc.Meta[key]; !ok || !truthy(v) {
				findings = append(findings, Finding{"error", "frontmatter", doc.Rel,
					fmt.Sprintf("missing required front-matter key %s", pyRepr(key))})
			}
		}
		if st, ok := doc.Meta["status"]; len(statuses) > 0 && ok && truthy(st) && !contains(statuses, doc.Status()) {
			findings = append(findings, Finding{"error", "frontmatter", doc.Rel,
				fmt.Sprintf("status %s not in allowed %s", pyRepr(doc.Status()), pyListRepr(statuses))})
		}
	}
	return findings
}

func checkLinks(model *DocsModel, cfg map[string]any) []Finding {
	sev := "warn"
	if checkBool(cfg, "broken_links_is_error", true) {
		sev = "error"
	}
	var findings []Finding
	for _, doc := range model.Documents {
		docDir := filepath.Dir(doc.Path)
		for _, link := range doc.Links() {
			target := strings.TrimSpace(strings.SplitN(link, "#", 2)[0])
			if target == "" || isExternal(target) {
				continue
			}
			resolved := filepath.Clean(filepath.Join(docDir, target))
			if !pathExists(resolved) {
				findings = append(findings, Finding{sev, "broken_link", doc.Rel,
					"link target does not exist: " + link})
			}
		}
	}
	return findings
}

func checkRelatedCode(model *DocsModel, cfg map[string]any) []Finding {
	orphanSev := "warn"
	if checkBool(cfg, "orphaned_code_is_error", true) {
		orphanSev = "error"
	}
	staleMode := checkStr(cfg, "staleness", "warn")
	ignore := toStringList(checkVal(cfg, "ignore_code_paths"))
	var findings []Finding
	for _, doc := range model.Documents {
		for _, codeRel := range doc.RelatedCode() {
			if isIgnored(codeRel, ignore) {
				continue
			}
			codePath := filepath.Join(model.Root, codeRel)
			if !pathExists(codePath) {
				findings = append(findings, Finding{orphanSev, "orphaned_code", doc.Rel,
					"related_code path not found: " + codeRel})
				continue
			}
			if staleMode != "off" && isStale(codePath, doc.Path) {
				findings = append(findings, Finding{staleMode, "staleness", doc.Rel,
					fmt.Sprintf("code %s is newer than its doc — review and "+
						"refresh, then bump the doc's mtime (re-save) "+
						"or update it", codeRel)})
			}
		}
	}
	return findings
}

// isStale reports whether the code file is more than 1s newer than its doc.
func isStale(codePath, docPath string) bool {
	ci, err := os.Stat(codePath)
	if err != nil {
		return false
	}
	di, err := os.Stat(docPath)
	if err != nil {
		return false
	}
	return ci.ModTime().Sub(di.ModTime()).Seconds() > 1.0
}

func checkDrift(model *DocsModel, cfg map[string]any, root string) []Finding {
	sev := "warn"
	if checkBool(cfg, "drift_is_error", true) {
		sev = "error"
	}
	var findings []Finding
	for _, art := range expectedArtifacts(model, cfg, root) {
		path := filepath.Join(root, art.rel)
		actual := readFileOrEmpty(path)
		if normalizeText(actual) != normalizeText(art.content) {
			reason := "out of date — run `maat sync`"
			if actual == "" {
				reason = "missing — run `maat sync`"
			}
			findings = append(findings, Finding{sev, "drift", art.rel, reason})
		}
	}
	return findings
}

// normalizeText strips trailing whitespace per line and surrounding blank lines
// so cosmetic differences don't register as drift.
func normalizeText(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t\r\n\v\f")
	}
	return strings.Trim(strings.Join(lines, "\n"), " \t\r\n\v\f")
}

// RunAll runs every check and returns findings sorted errors-first, then by
// location and code.
func RunAll(model *DocsModel, cfg map[string]any, root string) []Finding {
	var findings []Finding
	findings = append(findings, checkFrontmatter(model, cfg)...)
	findings = append(findings, checkLinks(model, cfg)...)
	findings = append(findings, checkRelatedCode(model, cfg)...)
	findings = append(findings, checkDrift(model, cfg, root)...)
	sort.SliceStable(findings, func(i, j int) bool {
		si, sj := severityRank(findings[i].Severity), severityRank(findings[j].Severity)
		if si != sj {
			return si < sj
		}
		if findings[i].Where != findings[j].Where {
			return findings[i].Where < findings[j].Where
		}
		return findings[i].Code < findings[j].Code
	})
	return findings
}

func severityRank(sev string) int {
	if sev == "error" {
		return 0
	}
	return 1
}
