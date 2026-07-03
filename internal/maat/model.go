package maat

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// sectionOrder fixes the order sections appear in generated indexes.
var sectionOrder = []string{"architecture", "decisions", "guides", "reference", "meta"}

// sectionTitles maps a section directory to its human heading.
var sectionTitles = map[string]string{
	"architecture": "Architecture — how the system is built",
	"decisions":    "Decisions — why it is built that way (ADRs)",
	"guides":       "Guides — how to work on it",
	"reference":    "Reference — factual surface (config, env, API)",
	"meta":         "Meta — conventions and glossary",
}

var (
	linkRe = regexp.MustCompile(`\[[^\]]*\]\(([^)]+)\)`)
	h1Re   = regexp.MustCompile(`(?m)^#\s+(.+?)\s*$`)
)

// Document is a single Markdown document within the docs tree.
type Document struct {
	Path string
	Rel  string // e.g. docs/guides/testing.md
	Meta map[string]any
	Body string
}

func newDocument(path, root string, meta map[string]any, body string) *Document {
	return &Document{Path: path, Rel: relPath(path, root), Meta: meta, Body: body}
}

// Section returns the section directory (e.g. "guides") or "_root" for
// top-level docs like index.md.
func (d *Document) Section() string {
	parts := strings.Split(d.Rel, "/")
	if len(parts) > 2 {
		return parts[1]
	}
	return "_root"
}

// Title prefers front-matter title, then the first H1, then the file stem.
func (d *Document) Title() string {
	if t, ok := d.Meta["title"]; ok && truthy(t) {
		return AnyToStr(t)
	}
	if m := h1Re.FindStringSubmatch(d.Body); m != nil {
		return m[1]
	}
	base := filepath.Base(d.Rel)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// Summary prefers front-matter summary, then the first non-heading,
// non-blockquote line of the body.
func (d *Document) Summary() string {
	if s, ok := d.Meta["summary"]; ok && truthy(s) {
		return AnyToStr(s)
	}
	for _, line := range strings.Split(d.Body, "\n") {
		stripped := strings.TrimSpace(line)
		if stripped != "" && !strings.HasPrefix(stripped, "#") && !strings.HasPrefix(stripped, ">") {
			return stripped
		}
	}
	return ""
}

// Status returns the front-matter status, defaulting to "current".
func (d *Document) Status() string {
	if s, ok := d.Meta["status"]; ok {
		return AnyToStr(s)
	}
	return "current"
}

// RelatedCode returns the related_code list (a bare string becomes one entry).
func (d *Document) RelatedCode() []string {
	v, ok := d.Meta["related_code"]
	if !ok {
		return nil
	}
	return toStringList(v)
}

// Links returns all Markdown link targets in the body.
func (d *Document) Links() []string {
	matches := linkRe.FindAllStringSubmatch(d.Body, -1)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		out = append(out, m[1])
	}
	return out
}

// DocsModel is the full documentation set, indexed for generation and
// validation. Both sync and check consume it, so they can never disagree about
// what the docs set contains.
type DocsModel struct {
	Root      string
	DocsDir   string
	DocsPath  string
	Documents []*Document
}

// ScanModel walks the docs directory, parses front-matter, and returns the
// model. Underscore-prefixed files are treated as templates/partials and
// skipped (they stay off indexes and out of validation but remain valid link
// targets on disk).
func ScanModel(root, docsDir string) (*DocsModel, error) {
	model := &DocsModel{Root: root, DocsDir: docsDir, DocsPath: filepath.Join(root, docsDir)}
	info, err := os.Stat(model.DocsPath)
	if err != nil || !info.IsDir() {
		return model, nil
	}
	// Collect files first so we can walk directories in the same order Python's
	// os.walk yields them, sorting filenames within each directory.
	type entry struct {
		dir   string
		names []string
	}
	dirs := map[string][]string{}
	var dirOrder []string
	err = filepath.WalkDir(model.DocsPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if _, seen := dirs[path]; !seen {
				dirs[path] = nil
				dirOrder = append(dirOrder, path)
			}
			return nil
		}
		dir := filepath.Dir(path)
		dirs[dir] = append(dirs[dir], d.Name())
		return nil
	})
	if err != nil {
		return nil, err
	}
	for _, dir := range dirOrder {
		names := dirs[dir]
		sort.Strings(names)
		for _, name := range names {
			if !strings.HasSuffix(name, ".md") {
				continue
			}
			if strings.HasPrefix(name, "_") {
				continue
			}
			full := filepath.Join(dir, name)
			meta, body, ferr := fmRead(full)
			if ferr != nil {
				return nil, ferr
			}
			model.Documents = append(model.Documents, newDocument(full, root, meta, body))
		}
	}
	sort.SliceStable(model.Documents, func(i, j int) bool {
		return model.Documents[i].Rel < model.Documents[j].Rel
	})
	return model, nil
}

// bySection groups documents by their section directory.
func (m *DocsModel) bySection() map[string][]*Document {
	buckets := map[string][]*Document{}
	for _, doc := range m.Documents {
		buckets[doc.Section()] = append(buckets[doc.Section()], doc)
	}
	return buckets
}

// Find returns the document with the given rel path, or nil.
func (m *DocsModel) Find(rel string) *Document {
	for _, doc := range m.Documents {
		if doc.Rel == rel {
			return doc
		}
	}
	return nil
}

// orderedSections returns present sections in canonical order, with any extras
// (non-standard section dirs) sorted after and "_root" excluded.
func (m *DocsModel) orderedSections() []string {
	present := m.bySection()
	var ordered []string
	for _, s := range sectionOrder {
		if _, ok := present[s]; ok {
			ordered = append(ordered, s)
		}
	}
	var extras []string
	for s := range present {
		if s == "_root" || contains(sectionOrder, s) {
			continue
		}
		extras = append(extras, s)
	}
	sort.Strings(extras)
	return append(ordered, extras...)
}

func sectionTitle(section string) string {
	if t, ok := sectionTitles[section]; ok {
		return t
	}
	return strings.Title(strings.ReplaceAll(section, "_", " ")) //nolint:staticcheck
}
