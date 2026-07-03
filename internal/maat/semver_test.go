package maat

import (
	"strings"
	"testing"
)

// --------------------------------------------------------------------------- #
// semver core
// --------------------------------------------------------------------------- #

func TestCompareSemver(t *testing.T) {
	mk := func(s string) semver { v, _ := parseSemver(s); return v }
	cases := []struct {
		a, b string
		want int
	}{
		{"0.3.0", "0.3.0", 0},
		{"0.3.1", "0.3.0", 1},
		{"0.3.0", "0.4.0", -1},
		{"1.0.0", "0.9.9", 1},
		{"0.3", "0.3.0", 0},   // missing patch defaults to 0
		{"1", "1.0.0", 0},     // missing minor+patch default to 0
		{"0.3.0-rc1", "0.3.0", -1}, // prerelease < release
		{"0.3.0-rc2", "0.3.0-rc1", 1},
	}
	for _, c := range cases {
		if got := compareSemver(mk(c.a), mk(c.b)); got != c.want {
			t.Errorf("compareSemver(%s,%s)=%d want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestParseSemverRejectsGarbage(t *testing.T) {
	for _, s := range []string{"dev", "", "abc", "1.2.x", "v"} {
		if _, ok := parseSemver(s); ok {
			t.Errorf("parseSemver(%q) should have failed", s)
		}
	}
}

// --------------------------------------------------------------------------- #
// dev-build detection (the exemption that keeps contributors unblocked)
// --------------------------------------------------------------------------- #

func TestIsDevBuild(t *testing.T) {
	dev := []string{
		"dev", "", "(devel)",
		"0.20260703120000-abcdef123456", // VCS pseudo-version
		"0.3.0+dirty",                   // build metadata
		"v0.0.0-20260101000000-000000000000",
		"garbage",
	}
	for _, s := range dev {
		if !isDevBuild(s) {
			t.Errorf("isDevBuild(%q)=false, want true", s)
		}
	}
	release := []string{"0.3.0", "v0.3.0", "1.2.3", "0.2.0-rc1"}
	for _, s := range release {
		if isDevBuild(s) {
			t.Errorf("isDevBuild(%q)=true, want false", s)
		}
	}
}

// --------------------------------------------------------------------------- #
// constraint parsing + satisfaction
// --------------------------------------------------------------------------- #

func TestConstraintSatisfaction(t *testing.T) {
	cases := []struct {
		constraint string
		version    string
		want       bool
	}{
		// pessimistic, two-component: >=0.3.0, <1.0.0
		{"~> 0.3", "0.3.0", true},
		{"~> 0.3", "0.9.9", true},
		{"~> 0.3", "0.2.9", false},
		{"~> 0.3", "1.0.0", false},
		// pessimistic, three-component: >=0.3.1, <0.4.0
		{"~> 0.3.1", "0.3.1", true},
		{"~> 0.3.1", "0.3.9", true},
		{"~> 0.3.1", "0.3.0", false},
		{"~> 0.3.1", "0.4.0", false},
		// comparators
		{">= 0.3.0", "0.3.0", true},
		{">= 0.3.0", "0.2.9", false},
		{"> 0.3.0", "0.3.0", false},
		{"<= 0.3.0", "0.3.0", true},
		{"< 0.4.0", "0.4.0", false},
		// exact
		{"0.3.0", "0.3.0", true},
		{"= 0.3.0", "0.3.1", false},
		// compound (comma = AND)
		{">= 0.3.0, < 0.5.0", "0.4.0", true},
		{">= 0.3.0, < 0.5.0", "0.5.0", false},
		{">= 0.3.0, < 0.5.0", "0.2.0", false},
	}
	for _, c := range cases {
		preds, err := parseConstraint(c.constraint)
		if err != nil {
			t.Fatalf("parseConstraint(%q) error: %v", c.constraint, err)
		}
		rv, _ := parseSemver(c.version)
		if got := satisfies(rv, preds); got != c.want {
			t.Errorf("satisfies(%s, %q)=%v want %v", c.version, c.constraint, got, c.want)
		}
	}
}

func TestParseConstraintRejectsGarbage(t *testing.T) {
	for _, expr := range []string{"", "   ", ">= ", "~> x", ">= 1, ", "abc"} {
		if _, err := parseConstraint(expr); err == nil {
			t.Errorf("parseConstraint(%q) should have errored", expr)
		}
	}
}

// checkVersionConstraint: dev builds are exempt (enforced=false) even when the
// version would otherwise violate the constraint.
func TestCheckVersionConstraintExemptsDevBuilds(t *testing.T) {
	enforced, ok, err := checkVersionConstraint("dev", "~> 99.0")
	if err != nil {
		t.Fatal(err)
	}
	if enforced {
		t.Error("dev build should be exempt (enforced=false)")
	}
	if !ok {
		t.Error("exempt build should report ok=true")
	}
	// A real release that violates the constraint IS enforced and fails.
	enforced, ok, err = checkVersionConstraint("0.1.0", "~> 0.3")
	if err != nil {
		t.Fatal(err)
	}
	if !enforced || ok {
		t.Errorf("release 0.1.0 vs ~>0.3 should be enforced+failing, got enforced=%v ok=%v", enforced, ok)
	}
}

// --------------------------------------------------------------------------- #
// end-to-end: enforcement through the CLI + config validation
// --------------------------------------------------------------------------- #

// TestMaatVersionValidatedInConfig ensures a malformed constraint fails config
// loading with exit 2 for everyone, dev build or not.
func TestMaatVersionBadConstraintExits2(t *testing.T) {
	root := initRepo(t)
	// Append to the scaffolded config so project_name/summary survive (a bare
	// overwrite would drop them and drift llms.txt, masking the real assertion).
	writeFile(t, root, ".maat.yml", readFile(t, root, ".maat.yml")+"maat_version: \"~> nonsense\"\n")
	_, errOut, code := run(t, "check", root)
	if code != 2 {
		t.Fatalf("expected exit 2 for bad maat_version, got %d", code)
	}
	if !strings.Contains(errOut, "maat_version") {
		t.Errorf("expected maat_version validation error, got: %s", errOut)
	}
}

// TestMaatVersionSatisfiedIsClean: because the test binary reports a dev
// version, even an unsatisfiable-looking constraint must be exempted, so check
// still runs to completion (exit 0 on the clean scaffold).
func TestMaatVersionExemptForDevBuild(t *testing.T) {
	root := initRepo(t)
	writeFile(t, root, ".maat.yml", readFile(t, root, ".maat.yml")+"maat_version: \"~> 999.0\"\n")
	out, _, code := run(t, "check", root)
	if code != 0 {
		t.Fatalf("dev build should be exempt from enforcement, got exit %d:\n%s", code, out)
	}
}

// TestEnforceVersionFailsForReleaseMismatch drives enforceVersion directly with
// a pinned release version to prove the failure path (the CLI path can't set
// the compiled-in version at runtime).
func TestEnforceVersionFailsForReleaseMismatch(t *testing.T) {
	old := version
	version = "0.1.0"
	defer func() { version = old }()

	err := enforceVersion(map[string]any{"maat_version": "~> 0.3"})
	if err == nil {
		t.Fatal("expected enforcement error for 0.1.0 vs ~> 0.3")
	}
	if !strings.Contains(err.Error(), "requires maat") {
		t.Errorf("expected upgrade hint, got: %v", err)
	}

	// Satisfying version → no error.
	version = "0.3.4"
	if err := enforceVersion(map[string]any{"maat_version": "~> 0.3"}); err != nil {
		t.Errorf("0.3.4 should satisfy ~> 0.3, got: %v", err)
	}

	// No constraint → no error.
	if err := enforceVersion(map[string]any{}); err != nil {
		t.Errorf("absent constraint should never error, got: %v", err)
	}
}
