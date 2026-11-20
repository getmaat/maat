package maat

import (
	"os"
	"path/filepath"
	"strings"
)

// readmeBadge is the Markdown badge `init` offers to add to a repository's
// README. It is a static shields.io badge: it needs no repo identity, no CI
// provider, and no network call from Ma'at, and it links readers to the
// project. The colour is Ma'at's brand teal.
const readmeBadge = "[![Docs checked by Ma'at](https://img.shields.io/badge/docs-checked_by_maat-03cdb4)](https://github.com/getmaat/maat)"

// badgeMarker identifies the badge in an existing README, so init neither
// offers it twice nor inserts a duplicate. Only the image URL is matched, so a
// README that links to Ma'at for other reasons is not mistaken for one that
// already shows the badge.
const badgeMarker = "img.shields.io/badge/docs-checked_by_maat"

// readmeFilename is the only README init will touch. Other spellings are left
// alone; the printed hint still tells their owners how to add the badge.
const readmeFilename = "README.md"

// readmeBadgeState reports whether root has a README.md and whether it already
// carries the badge.
func readmeBadgeState(root string) (exists, hasBadge bool) {
	data, err := os.ReadFile(filepath.Join(root, readmeFilename))
	if err != nil {
		return false, false
	}
	return true, strings.Contains(string(data), badgeMarker)
}

// addReadmeBadge inserts readmeBadge into root's README.md: on its own line
// after the first top-level "# " heading near the top of the file, or at the
// very top when there is none. It is opt-in only (the init wizard asks), never
// creates a README, and is a no-op when the badge is already there. It
// reports whether the file was changed.
func addReadmeBadge(root string) (bool, error) {
	path := filepath.Join(root, readmeFilename)
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	content := string(data)
	if strings.Contains(content, badgeMarker) {
		return false, nil
	}
	if err := os.WriteFile(path, []byte(insertBadge(content)), 0o644); err != nil {
		return false, err
	}
	return true, nil
}

// headingSearchLines bounds how far down insertBadge looks for the title
// heading. A "# " line further down is a section, not the title, and putting a
// badge there would look misplaced.
const headingSearchLines = 10

// insertBadge returns content with readmeBadge placed below the title heading
// (blank line on either side), or prepended when no title heading is found.
func insertBadge(content string) string {
	lines := strings.Split(content, "\n")
	for i := 0; i < len(lines) && i < headingSearchLines; i++ {
		if !strings.HasPrefix(lines[i], "# ") {
			continue
		}
		rest := lines[i+1:]
		// Drop blank lines right after the heading; they are re-added below
		// so the result has exactly one blank line on each side of the badge.
		for len(rest) > 0 && strings.TrimSpace(rest[0]) == "" {
			rest = rest[1:]
		}
		out := append([]string{}, lines[:i+1]...)
		out = append(out, "", readmeBadge)
		if len(rest) > 0 {
			out = append(out, "")
			out = append(out, rest...)
		} else {
			out = append(out, "")
		}
		return strings.Join(out, "\n")
	}
	return readmeBadge + "\n\n" + content
}
