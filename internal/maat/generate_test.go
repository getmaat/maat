package maat

import (
	"strings"
	"testing"
)

// contractHeading is kept verbatim in the generated block because the adapter
// pointers cross-reference the instruction file "under this heading".
const contractHeading = "## Documentation update protocol"

// TestContractBlockCarriesTheInvariants verifies the maintenance-contract block
// (ADR 0009) renders the update protocol, the front-matter schema, and the
// skills list as one managed section, and parameterizes its paths on docsDir.
func TestContractBlockCarriesTheInvariants(t *testing.T) {
	block := contractBlock(skillDefs, "docs")
	for _, want := range []string{
		contractHeading,
		"A change is not complete until",
		"### Front-matter every doc carries",
		"## Skills (reusable procedures)",
		"docs/architecture/",
		"docs/decisions/",
	} {
		if !strings.Contains(block, want) {
			t.Errorf("contractBlock missing %q\n---\n%s", want, block)
		}
	}
	// The skills list is present (retrospect ships by default, ADR 0007/0008).
	if len(skillDefs) > 0 && !strings.Contains(block, "/SKILL.md)") {
		t.Errorf("contractBlock should list at least one skill\n---\n%s", block)
	}

	// A non-default docs directory must be reflected in the paths — no
	// hardcoded "docs/" leaks through.
	custom := contractBlock(skillDefs, "documentation")
	if !strings.Contains(custom, "documentation/architecture/") {
		t.Errorf("contractBlock did not honor custom docs_dir\n---\n%s", custom)
	}
	if strings.Contains(custom, "docs/architecture/") {
		t.Errorf("contractBlock leaked hardcoded docs/ path with custom docs_dir\n---\n%s", custom)
	}
}

// TestGreenfieldInstructionFileGetsContractOnce verifies a fresh init produces
// exactly one copy of the contract, inside the managed markers — the protocol
// is generated content now, not hand-written scaffold prose (ADR 0009).
func TestGreenfieldInstructionFileGetsContractOnce(t *testing.T) {
	root := initRepo(t)
	agents := readFile(t, root, "AGENTS.md")

	if n := strings.Count(agents, contractHeading); n != 1 {
		t.Fatalf("expected the update protocol exactly once, got %d\n---\n%s", n, agents)
	}
	if !strings.Contains(agents, beginMarker) || !strings.Contains(agents, endMarker) {
		t.Fatalf("instruction file is missing the managed markers\n---\n%s", agents)
	}
	// The heading must live inside the managed region, not the hand-written body.
	managed := agents[strings.Index(agents, beginMarker):strings.Index(agents, endMarker)]
	if !strings.Contains(managed, contractHeading) {
		t.Errorf("update protocol should be inside the managed block, not the body\n---\n%s", agents)
	}
}

// TestBrownfieldInstructionFileGainsContractNonDestructively is the core of ADR
// 0009: an AGENTS.md that already exists is preserved (skipped as a scaffold
// file) yet still gains Ma'at's contract, spliced in without disturbing the
// team's hand-written text.
func TestBrownfieldInstructionFileGainsContractNonDestructively(t *testing.T) {
	root := t.TempDir()
	const handWritten = "# Our House Rules\n\nWe deploy on Fridays and we like it.\n"
	writeFile(t, root, "AGENTS.md", handWritten)

	stdout, _, code := run(t, "init", root, "--name", "Legacy", "--summary", "An old repo.")
	if code != 0 {
		t.Fatalf("init exited %d\n%s", code, stdout)
	}

	// The pre-existing file is reported as skipped (scaffold) *and* generated
	// (the block was spliced in) — see ADR 0009 / the init guidance.
	if !strings.Contains(stdout, "skip    AGENTS.md") {
		t.Errorf("expected AGENTS.md to be reported as skipped\n%s", stdout)
	}
	if !strings.Contains(stdout, "gen     AGENTS.md") {
		t.Errorf("expected AGENTS.md to be reported as generated (block spliced)\n%s", stdout)
	}
	if !strings.Contains(stdout, "maintenance contract") {
		t.Errorf("brownfield guidance should mention the injected maintenance contract\n%s", stdout)
	}

	agents := readFile(t, root, "AGENTS.md")
	// Hand-written content is preserved verbatim.
	if !strings.Contains(agents, "We deploy on Fridays and we like it.") {
		t.Errorf("hand-written content was not preserved\n---\n%s", agents)
	}
	// The contract was appended, inside the managed markers.
	if !strings.Contains(agents, contractHeading) {
		t.Errorf("brownfield AGENTS.md never gained the update protocol\n---\n%s", agents)
	}
	if !strings.Contains(agents, "## Skills (reusable procedures)") {
		t.Errorf("brownfield AGENTS.md never gained the skills index\n---\n%s", agents)
	}
	if !strings.HasPrefix(agents, "# Our House Rules") {
		t.Errorf("the managed block should be appended, not prepended\n---\n%s", agents)
	}
}

// TestBrownfieldContractInjectionIsIdempotent verifies re-running init on a
// brownfield repo does not duplicate the contract: the second run's sync finds
// the markers and replaces the region in place.
func TestBrownfieldContractInjectionIsIdempotent(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "AGENTS.md", "# Legacy\n\nHand-written.\n")

	for i := 0; i < 2; i++ {
		if _, _, code := run(t, "init", root, "--name", "Legacy", "--summary", "x"); code != 0 {
			t.Fatalf("init run %d exited %d", i, code)
		}
	}
	agents := readFile(t, root, "AGENTS.md")
	if n := strings.Count(agents, contractHeading); n != 1 {
		t.Fatalf("re-running init duplicated the contract: %d copies\n---\n%s", n, agents)
	}
	if n := strings.Count(agents, beginMarker); n != 1 {
		t.Fatalf("re-running init duplicated the managed markers: %d copies\n---\n%s", n, agents)
	}

	// And a plain re-sync after that is a no-op for the instruction file.
	out, _, code := run(t, "sync", root)
	if code != 0 {
		t.Fatalf("sync exited %d\n%s", code, out)
	}
	if strings.Contains(out, "AGENTS.md") {
		t.Errorf("a settled instruction file should not be rewritten by sync\n%s", out)
	}
}
