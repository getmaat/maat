package maat

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const fmFence = "---"

// FrontMatterError is raised when a document's front-matter is malformed
// (opened but never closed, or not a mapping).
type FrontMatterError struct{ msg string }

func (e *FrontMatterError) Error() string { return e.msg }

// fmSplit returns (metadata, body) for a Markdown document. A document with no
// front-matter yields an empty map and the whole text as body. A fence that
// opens but never closes is an error, so authors notice typos early.
func fmSplit(text string) (map[string]any, string, error) {
	probe := strings.TrimLeft(text, "\ufeff")
	leadingWs := text[:len(text)-len(probe)]
	if !strings.HasPrefix(probe, fmFence+"\n") && strings.TrimSpace(probe) != fmFence {
		return map[string]any{}, text, nil
	}
	lines := strings.Split(probe, "\n")
	if strings.TrimSpace(lines[0]) != fmFence {
		return map[string]any{}, text, nil
	}
	closing := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == fmFence {
			closing = i
			break
		}
	}
	if closing == -1 {
		return nil, "", &FrontMatterError{"front-matter opened with '---' but never closed"}
	}
	rawMeta := strings.Join(lines[1:closing], "\n")
	body := strings.Join(lines[closing+1:], "\n")
	if strings.HasPrefix(body, "\n") {
		body = body[1:]
	}
	var meta any = map[string]any{}
	if strings.TrimSpace(rawMeta) != "" {
		parsed, err := yamlParse(rawMeta)
		if err != nil {
			return nil, "", err
		}
		meta = parsed
	}
	metaMap, ok := meta.(map[string]any)
	if !ok {
		return nil, "", &FrontMatterError{fmt.Sprintf("front-matter must be a mapping, got %T", meta)}
	}
	cleanLeading := strings.ReplaceAll(leadingWs, "\ufeff", "")
	return metaMap, cleanLeading + body, nil
}

// fmRead reads a file and splits it into (metadata, body).
func fmRead(path string) (map[string]any, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	return fmSplit(string(data))
}

// relPath returns a POSIX-style path relative to root (stable across OSes).
func relPath(path, root string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		rel = path
	}
	return filepath.ToSlash(rel)
}
