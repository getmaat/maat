// Package maat is the Go implementation of the Ma'at engine: scanning a
// docs/ tree into a model, generating derived artifacts (llms.txt, index
// navigation, agent adapter files), and validating the set for CI.
//
// It is a faithful port of the reference Python implementation under codedoc/.
// The two produce byte-identical output; the Python test suite and this
// package's tests are the shared conformance spec.
package maat

import (
	"os"
	"runtime/debug"
	"strconv"
	"strings"
)

// version is the release version, injected at build time by GoReleaser via
// -ldflags "-X github.com/getmaat/maat/internal/maat.version=<tag>". It is
// empty for non-release builds.
var version = ""

// Version returns the CLI version string. Resolution order:
//  1. the release version injected at build time via -ldflags (GoReleaser
//     sets this for the published binaries);
//  2. the module version from the build info — a clean tag such as "v0.1.0"
//     when installed with `go install <module>@vX`, or a VCS pseudo-version
//     (commit + "+dirty") for a source build ahead of / off a release tag;
//  3. "dev" when no build info is available at all.
func Version() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return strings.TrimPrefix(v, "v")
		}
	}
	return "dev"
}

// truthy mirrors Python's notion of truthiness for the scalar/collection types
// our YAML subset yields. Used wherever the Python code relied on `if value:`.
func truthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case string:
		return x != ""
	case int64:
		return x != 0
	case float64:
		return x != 0
	case []any:
		return len(x) > 0
	case map[string]any:
		return len(x) > 0
	default:
		return true
	}
}

// AnyToStr mirrors Python str() for the values our YAML parser produces. It is
// exported because the CLI needs it to read scalar config keys (e.g. docs_dir).
func AnyToStr(v any) string {
	switch x := v.(type) {
	case nil:
		return "None"
	case bool:
		if x {
			return "True"
		}
		return "False"
	case string:
		return x
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		return strconv.FormatFloat(x, 'g', -1, 64)
	default:
		return ""
	}
}

// toStringList coerces a parsed YAML value into a []string, matching how the
// Python code treats list-valued config/front-matter keys. A bare string
// becomes a single-element list (mirrors related_code's str handling).
func toStringList(v any) []string {
	switch x := v.(type) {
	case []any:
		out := make([]string, 0, len(x))
		for _, e := range x {
			out = append(out, AnyToStr(e))
		}
		return out
	case []string:
		return x
	case string:
		return []string{x}
	default:
		return nil
	}
}

func contains(list []string, s string) bool {
	for _, e := range list {
		if e == s {
			return true
		}
	}
	return false
}

func pathExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func readFileOrEmpty(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

// pySplitlines approximates Python str.splitlines() for the line endings our
// inputs use (\n and \r\n). A trailing newline does not yield a final empty
// element in Python; here it does, but every caller skips blank lines so the
// observable behaviour is identical.
func pySplitlines(text string) []string {
	if text == "" {
		return nil
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	return strings.Split(text, "\n")
}

// pyRepr / pyListRepr reproduce Python's %r for the strings and string lists
// that appear in check findings, so messages read the same across ports.
func pyRepr(s string) string { return "'" + s + "'" }

func pyListRepr(list []string) string {
	parts := make([]string, len(list))
	for i, s := range list {
		parts[i] = "'" + s + "'"
	}
	return "[" + strings.Join(parts, ", ") + "]"
}
