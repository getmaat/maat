package maat

import (
	"strings"
	"testing"
)

// setVersion overrides the package-level release version for the duration of a
// test and restores it afterward, so we can exercise both the release and
// development scaffolding branches deterministically without -ldflags.
func setVersion(t *testing.T, v string) {
	t.Helper()
	prev := version
	version = v
	t.Cleanup(func() { version = prev })
}

// TestScaffoldVersionPinAndActionRef locks in the ADR 0006 stamping contract:
// a release build pins its own exact version/tag into generated CI, while a
// development build emits an empty MAAT_VERSION (track latest) and an obvious
// vX.Y.Z placeholder — never a broken "@v" from an empty substitution.
func TestScaffoldVersionPinAndActionRef(t *testing.T) {
	cases := []struct {
		name    string
		version string
		wantPin string
		wantRef string
	}{
		{"release", "0.2.0", "0.2.0", "v0.2.0"},
		{"release_prerelease", "1.0.0-rc1", "1.0.0-rc1", "v1.0.0-rc1"},
		{"dev_empty", "", "", "vX.Y.Z"},
		{"dev_literal", "dev", "", "vX.Y.Z"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setVersion(t, tc.version)
			if got := scaffoldVersionPin(); got != tc.wantPin {
				t.Errorf("scaffoldVersionPin() = %q, want %q", got, tc.wantPin)
			}
			if got := scaffoldActionRef(); got != tc.wantRef {
				t.Errorf("scaffoldActionRef() = %q, want %q", got, tc.wantRef)
			}
		})
	}
}

// TestScaffoldedWorkflowPinsReleaseRefs verifies that a release build renders a
// self-consistent CI workflow: the exact version in MAAT_VERSION and the exact
// tag in every `uses:` ref. A stale @v1 (which would collide with the semver
// release trigger) or an unrendered {{...}} placeholder must never appear.
func TestScaffoldedWorkflowPinsReleaseRefs(t *testing.T) {
	setVersion(t, "0.2.0")
	root := t.TempDir()
	if _, _, code := run(t, "init", root, "--name", "Demo", "--summary", "x"); code != 0 {
		t.Fatalf("init exited %d", code)
	}
	wf := readFile(t, root, ".github/workflows/maat.yml")

	for _, want := range []string{
		`MAAT_VERSION: "0.2.0"`,
		"getmaat/maat@v0.2.0",
		"maat-check.yml@v0.2.0",
	} {
		if !strings.Contains(wf, want) {
			t.Errorf("scaffolded workflow missing %q\n---\n%s", want, wf)
		}
	}
	for _, bad := range []string{"@v1", "{{", "@v\"", "@v "} {
		if strings.Contains(wf, bad) {
			t.Errorf("scaffolded workflow contains forbidden %q\n---\n%s", bad, wf)
		}
	}
}

// TestScaffoldedWorkflowDevBuildTracksLatest verifies the development-build
// branch: MAAT_VERSION is empty (track latest) and the Action refs render the
// vX.Y.Z placeholder rather than a broken empty "@v".
func TestScaffoldedWorkflowDevBuildTracksLatest(t *testing.T) {
	setVersion(t, "") // build info yields "dev" under `go test`
	root := t.TempDir()
	if _, _, code := run(t, "init", root, "--name", "Demo", "--summary", "x"); code != 0 {
		t.Fatalf("init exited %d", code)
	}
	wf := readFile(t, root, ".github/workflows/maat.yml")

	if !strings.Contains(wf, `MAAT_VERSION: ""`) {
		t.Errorf("dev-build workflow should leave MAAT_VERSION empty\n---\n%s", wf)
	}
	if !strings.Contains(wf, "getmaat/maat@vX.Y.Z") {
		t.Errorf("dev-build workflow should render the vX.Y.Z placeholder\n---\n%s", wf)
	}
	// Guard against the old bug: an empty ref substitution rendering "@v\n".
	for _, line := range strings.Split(wf, "\n") {
		trimmed := strings.TrimRight(line, " ")
		if strings.HasSuffix(trimmed, "@v") {
			t.Errorf("dev-build workflow rendered a broken empty ref: %q", line)
		}
	}
}
