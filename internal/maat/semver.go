package maat

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// This file implements the small slice of Semantic Versioning that the
// `maat_version` config constraint needs (see ADR 0006). It is deliberately
// dependency-free — like the YAML subset, we own a minimal implementation
// rather than pull in a module, to keep the "single static binary, no
// dependencies" promise (ADR 0005).
//
// Two jobs live here:
//   1. Parse the running binary's version and decide whether it is a real
//      release we should enforce a constraint against, or a development build
//      we must exempt (source builds, `go run`, VCS pseudo-versions).
//   2. Parse a `maat_version` constraint expression and test whether the
//      running version satisfies it.

// semver is a parsed semantic version. Only the numeric core participates in
// ordering comparisons for constraint bounds; prerelease is retained so a
// prerelease (e.g. 0.3.0-rc1) can rank below its release (0.3.0), matching
// SemVer §11.
type semver struct {
	major, minor, patch int
	prerelease          string
}

// coreRe matches a clean SemVer core with an optional simple prerelease. Build
// metadata (+...) is stripped before matching; a version carrying build
// metadata is treated as a development build (see isDevBuild).
var coreRe = regexp.MustCompile(`^(\d+)(?:\.(\d+))?(?:\.(\d+))?(?:-([0-9A-Za-z.-]+))?$`)

// pseudoTimestampRe / commitHashRe detect a Go module VCS pseudo-version's
// prerelease segment (e.g. "0.20260703120000-abcdef123456"): a 14-digit UTC
// timestamp and/or a 12-hex commit hash. Their presence marks a build made
// from source ahead of a tag, which we exempt from constraint enforcement.
var (
	pseudoTimestampRe = regexp.MustCompile(`\d{14}`)
	commitHashRe      = regexp.MustCompile(`\b[0-9a-f]{12,}\b`)
)

// parseSemver parses a clean "X", "X.Y", or "X.Y.Z" with an optional simple
// prerelease. Missing minor/patch default to 0. A leading "v" is tolerated.
// Build metadata must already be stripped by the caller. ok is false for
// anything that is not a clean core (e.g. "dev").
func parseSemver(s string) (semver, bool) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "v")
	m := coreRe.FindStringSubmatch(s)
	if m == nil {
		return semver{}, false
	}
	atoi := func(x string) int {
		if x == "" {
			return 0
		}
		n, _ := strconv.Atoi(x)
		return n
	}
	return semver{
		major:      atoi(m[1]),
		minor:      atoi(m[2]),
		patch:      atoi(m[3]),
		prerelease: m[4],
	}, true
}

// componentCount reports how many dot-separated numeric components a version
// token was written with (1, 2, or 3), which the "~>" operator needs to pick
// its upper bound. Returns 0 if the token is not a clean version.
func componentCount(s string) int {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "v")
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}
	if coreRe.FindStringSubmatch(s) == nil {
		return 0
	}
	return strings.Count(s, ".") + 1
}

// compareSemver returns -1, 0, or +1 comparing a to b: numeric core first,
// then SemVer prerelease precedence (a version WITH a prerelease is lower than
// the same core WITHOUT one; two prereleases compare lexically, which is
// sufficient for the simple rcN identifiers our tags use).
func compareSemver(a, b semver) int {
	for _, d := range []int{a.major - b.major, a.minor - b.minor, a.patch - b.patch} {
		if d != 0 {
			if d < 0 {
				return -1
			}
			return 1
		}
	}
	if a.prerelease == b.prerelease {
		return 0
	}
	if a.prerelease == "" {
		return 1 // release > prerelease
	}
	if b.prerelease == "" {
		return -1
	}
	return strings.Compare(a.prerelease, b.prerelease)
}

// isDevBuild reports whether the running version string denotes a development
// build that must be EXEMPT from constraint enforcement: the "dev" sentinel,
// anything unparseable as a clean core, a build carrying "+" metadata (a dirty
// or annotated build), or a Go VCS pseudo-version (timestamp + commit hash in
// the prerelease). Clean release tags — including prereleases like "0.2.0-rc1"
// — return false and ARE enforced.
func isDevBuild(running string) bool {
	r := strings.TrimSpace(running)
	if r == "" || r == "dev" || r == "(devel)" {
		return true
	}
	if strings.Contains(r, "+") {
		return true // build metadata: dirty / non-release build
	}
	rv, ok := parseSemver(r)
	if !ok {
		return true
	}
	if pseudoTimestampRe.MatchString(rv.prerelease) || commitHashRe.MatchString(rv.prerelease) {
		return true // VCS pseudo-version: source ahead of a tag
	}
	return false
}

// predicate is one comparison a version must satisfy: op in {>=,>,<=,<,=}.
type predicate struct {
	op  string
	ver semver
}

// parseConstraint parses a `maat_version` expression into a set of predicates
// that must ALL hold (comma-separated, Terraform-style). Supported terms:
//
//	~> X.Y      pessimistic: >= X.Y.0, < (X+1).0.0
//	~> X.Y.Z    pessimistic: >= X.Y.Z, < X.(Y+1).0
//	>= X.Y.Z    at least
//	>  X.Y.Z    strictly greater
//	<= X.Y.Z    at most
//	<  X.Y.Z    strictly less
//	=  X.Y.Z    exactly (also the meaning of a bare "X.Y.Z")
//
// It returns a clear error for empty or malformed expressions so a typo in
// .maat.yml fails config validation (exit 2) rather than silently misbehaving.
func parseConstraint(expr string) ([]predicate, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil, fmt.Errorf("empty version constraint")
	}
	var preds []predicate
	for _, raw := range strings.Split(expr, ",") {
		term := strings.TrimSpace(raw)
		if term == "" {
			return nil, fmt.Errorf("empty term in version constraint %s", pyRepr(expr))
		}
		// Operator is the leading run of comparison symbols / "~".
		op := ""
		for _, prefix := range []string{"~>", ">=", "<=", "=", ">", "<"} {
			if strings.HasPrefix(term, prefix) {
				op = prefix
				break
			}
		}
		verTok := strings.TrimSpace(strings.TrimPrefix(term, op))
		comps := componentCount(verTok)
		if comps == 0 {
			return nil, fmt.Errorf("invalid version %s in constraint %s", pyRepr(verTok), pyRepr(expr))
		}
		ver, ok := parseSemver(verTok)
		if !ok {
			return nil, fmt.Errorf("invalid version %s in constraint %s", pyRepr(verTok), pyRepr(expr))
		}
		switch op {
		case "~>":
			// Lower bound is the version as written; upper bound depends on
			// how precisely it was pinned.
			var upper semver
			if comps >= 3 {
				upper = semver{major: ver.major, minor: ver.minor + 1, patch: 0}
			} else {
				upper = semver{major: ver.major + 1, minor: 0, patch: 0}
			}
			preds = append(preds,
				predicate{">=", semver{ver.major, ver.minor, ver.patch, ""}},
				predicate{"<", upper},
			)
		case ">=", ">", "<=", "<":
			preds = append(preds, predicate{op, ver})
		case "=", "":
			preds = append(preds, predicate{"=", ver})
		}
	}
	return preds, nil
}

// satisfies reports whether running meets every predicate.
func satisfies(running semver, preds []predicate) bool {
	for _, p := range preds {
		c := compareSemver(running, p.ver)
		ok := false
		switch p.op {
		case ">=":
			ok = c >= 0
		case ">":
			ok = c > 0
		case "<=":
			ok = c <= 0
		case "<":
			ok = c < 0
		case "=":
			ok = c == 0
		}
		if !ok {
			return false
		}
	}
	return true
}

// checkVersionConstraint tests the running version string against a constraint
// expression. It returns:
//
//	enforced — whether the constraint was actually applied (false for a
//	           development build, which is always exempt);
//	ok       — whether the running version satisfies the constraint (only
//	           meaningful when enforced is true);
//	err      — a malformed constraint expression.
//
// Splitting parse errors from satisfaction lets the caller validate syntax for
// everyone while enforcing satisfaction only for real release binaries.
func checkVersionConstraint(running, expr string) (enforced, ok bool, err error) {
	preds, err := parseConstraint(expr)
	if err != nil {
		return false, false, err
	}
	if isDevBuild(running) {
		return false, true, nil
	}
	rv, _ := parseSemver(strings.TrimPrefix(strings.TrimSpace(running), "v"))
	return true, satisfies(rv, preds), nil
}
