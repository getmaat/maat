package maat

import (
	"encoding/json"
	"strings"
	"testing"
)

// --------------------------------------------------------------------------- #
// contract edges and manifest (ADR 0013)
// --------------------------------------------------------------------------- #

// contractDoc writes a module doc with the given front-matter lines added.
func contractDoc(t *testing.T, root, name, extra string) {
	t.Helper()
	writeFile(t, root, "docs/architecture/modules/"+name+".md",
		"---\ntitle: "+name+"\nstatus: current\nsummary: x\n"+extra+"---\n# "+name+"\n")
}

// syncAndCheck runs sync (so generated files are current) and then check.
func syncAndCheck(t *testing.T, root string) (string, int) {
	t.Helper()
	if _, _, code := run(t, "sync", root); code != 0 {
		t.Fatalf("sync exited %d", code)
	}
	out, _, code := run(t, "check", root)
	return out, code
}

func readManifest(t *testing.T, root string) manifest {
	t.Helper()
	var m manifest
	if err := json.Unmarshal([]byte(readFile(t, root, manifestPath)), &m); err != nil {
		t.Fatalf("manifest is not valid JSON: %v", err)
	}
	return m
}

func TestManifestEmptyByDefault(t *testing.T) {
	root := initRepo(t)
	if out, code := syncAndCheck(t, root); code != 0 {
		t.Fatalf("clean repo should pass, got %d:\n%s", code, out)
	}
	m := readManifest(t, root)
	if m.Schema != 1 || m.Repo != nil || len(m.Provides) != 0 || len(m.Consumes) != 0 {
		t.Errorf("unexpected empty manifest: %+v", m)
	}
	if strings.Contains(readFile(t, root, "docs/llms.txt"), "## Contracts") {
		t.Error("llms.txt should not have a Contracts section when nothing is declared")
	}
}

func TestManifestCollectsSortedEdges(t *testing.T) {
	root := initRepo(t)
	writeFile(t, root, ".maat.yml", "repo: acme/billing\n")
	writeFile(t, root, "api/openapi.yaml", "openapi: 3.1.0\n")
	contractDoc(t, root, "invoice", "provides:\n  - id: invoice-api\n    kind: http-api\n"+
		"    version: 2.1.0\n    spec: api/openapi.yaml\n  - id: audit-log\n")
	contractDoc(t, root, "client", "consumes:\n  - ref: invoice-api\n    version: \"~> 2.1\"\n"+
		"  - ref: acme/ledger#ledger-events\n")

	if out, code := syncAndCheck(t, root); code != 0 {
		t.Fatalf("valid edges should pass, got %d:\n%s", code, out)
	}
	m := readManifest(t, root)
	if m.Repo == nil || *m.Repo != "acme/billing" {
		t.Errorf("repo = %v, want acme/billing", m.Repo)
	}
	if len(m.Provides) != 2 || m.Provides[0].ID != "audit-log" || m.Provides[1].ID != "invoice-api" {
		t.Fatalf("provides not collected/sorted: %+v", m.Provides)
	}
	inv := m.Provides[1]
	if inv.Kind != "http-api" || inv.Version != "2.1.0" || inv.Spec != "api/openapi.yaml" ||
		inv.Doc != "docs/architecture/modules/invoice.md" {
		t.Errorf("invoice-api entry wrong: %+v", inv)
	}
	if len(m.Consumes) != 2 || m.Consumes[0].Ref != "acme/ledger#ledger-events" || m.Consumes[1].Ref != "invoice-api" {
		t.Errorf("consumes not collected/sorted: %+v", m.Consumes)
	}

	llms := readFile(t, root, "docs/llms.txt")
	for _, want := range []string{"## Contracts", "Provides `invoice-api` (http-api, 2.1.0)",
		"Consumes `acme/ledger#ledger-events`", "Consumes `invoice-api` (~> 2.1)"} {
		if !strings.Contains(llms, want) {
			t.Errorf("llms.txt missing %q:\n%s", want, llms)
		}
	}
}

func TestManifestDriftIsDetected(t *testing.T) {
	root := initRepo(t)
	contractDoc(t, root, "svc", "provides:\n  - id: svc-api\n")
	syncAndCheck(t, root)
	// Declaring a new contract without re-running sync leaves the manifest stale.
	contractDoc(t, root, "svc", "provides:\n  - id: svc-api\n  - id: svc-events\n")
	out, _, code := run(t, "check", root)
	if code != 1 || !strings.Contains(out, manifestPath) {
		t.Errorf("expected drift on %s, got exit %d:\n%s", manifestPath, code, out)
	}
}

func TestContractSchemaFindings(t *testing.T) {
	cases := []struct {
		name, frontMatter, want string
	}{
		{"missing id", "provides:\n  - kind: cli\n", "missing its id"},
		{"invalid id", "provides:\n  - id: Invoice_API\n", "must be lowercase"},
		{"unknown key", "provides:\n  - id: a\n    owner: me\n", "unknown key(s) owner"},
		{"not a mapping", "provides:\n  - a\n", "must be a mapping"},
		{"not a list", "provides: a\n", "must be a list"},
		{"bad semver", "provides:\n  - id: a\n    version: two\n", "invalid version"},
		{"unquoted two-part version", "provides:\n  - id: a\n    version: 1.10\n", "quote it"},
		{"missing ref", "consumes:\n  - version: \"1.0\"\n", "missing its ref"},
		{"bad cross-repo ref", "consumes:\n  - ref: nope#x\n", "owner/repo#id"},
		{"bad constraint", "consumes:\n  - ref: acme/x#y\n    version: \"~> banana\"\n", "invalid version"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := initRepo(t)
			contractDoc(t, root, "m", tc.frontMatter)
			out, code := syncAndCheck(t, root)
			if code != 1 || !strings.Contains(out, "contract_schema") || !strings.Contains(out, tc.want) {
				t.Errorf("expected contract_schema error containing %q, got exit %d:\n%s", tc.want, code, out)
			}
		})
	}
}

func TestContractDuplicateIDAcrossDocs(t *testing.T) {
	root := initRepo(t)
	contractDoc(t, root, "a", "provides:\n  - id: shared\n")
	contractDoc(t, root, "b", "provides:\n  - id: shared\n")
	out, code := syncAndCheck(t, root)
	if code != 1 || !strings.Contains(out, "duplicate contract id 'shared'") {
		t.Errorf("expected duplicate-id error, got exit %d:\n%s", code, out)
	}
	// The first declaration still lands in the manifest; the duplicate does not.
	if m := readManifest(t, root); len(m.Provides) != 1 {
		t.Errorf("manifest should keep exactly one 'shared': %+v", m.Provides)
	}
}

func TestContractUnresolvedLocalRef(t *testing.T) {
	root := initRepo(t)
	contractDoc(t, root, "client", "consumes:\n  - ref: missing-api\n")
	out, code := syncAndCheck(t, root)
	if code != 1 || !strings.Contains(out, "contract_unresolved") {
		t.Errorf("expected contract_unresolved, got exit %d:\n%s", code, out)
	}
}

func TestContractCrossRepoRefsAreNotResolved(t *testing.T) {
	root := initRepo(t)
	contractDoc(t, root, "client", "consumes:\n  - ref: acme/elsewhere#api\n    version: \"~> 9\"\n")
	if out, code := syncAndCheck(t, root); code != 0 {
		t.Errorf("cross-repo refs must not be resolved yet, got exit %d:\n%s", code, out)
	}
}

func TestContractVersionChecks(t *testing.T) {
	cases := []struct {
		name, provider, constraint string
		ok                         bool
		want                       string
	}{
		{"satisfied", "    version: 2.3.0\n", "~> 2.1", true, ""},
		{"too new", "    version: 3.0.0\n", "~> 2.1", false, "provides version 3.0.0"},
		{"too old", "    version: 1.9.0\n", ">= 2.0", false, "provides version 1.9.0"},
		{"provider has no version", "", "~> 2.1", false, "declares no version"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := initRepo(t)
			contractDoc(t, root, "svc", "provides:\n  - id: svc-api\n"+tc.provider)
			contractDoc(t, root, "client", "consumes:\n  - ref: svc-api\n    version: \""+tc.constraint+"\"\n")
			out, code := syncAndCheck(t, root)
			if tc.ok {
				if code != 0 {
					t.Errorf("expected pass, got exit %d:\n%s", code, out)
				}
				return
			}
			if code != 1 || !strings.Contains(out, "contract_version") || !strings.Contains(out, tc.want) {
				t.Errorf("expected contract_version containing %q, got exit %d:\n%s", tc.want, code, out)
			}
		})
	}
}

func TestContractsIsErrorFalseDowngrades(t *testing.T) {
	root := initRepo(t)
	writeFile(t, root, ".maat.yml", "check:\n  contracts_is_error: false\n")
	contractDoc(t, root, "client", "consumes:\n  - ref: missing-api\n")
	out, code := syncAndCheck(t, root)
	if code != 0 || !strings.Contains(out, "contract_unresolved") {
		t.Errorf("contracts_is_error: false should warn (exit 0), got exit %d:\n%s", code, out)
	}
}

func TestContractSpecIsRelatedCode(t *testing.T) {
	root := initRepo(t)
	contractDoc(t, root, "svc", "provides:\n  - id: svc-api\n    spec: api/missing.yaml\n")
	out, code := syncAndCheck(t, root)
	if code != 1 || !strings.Contains(out, "orphaned_code") || !strings.Contains(out, "api/missing.yaml") {
		t.Errorf("a missing spec should be orphaned_code, got exit %d:\n%s", code, out)
	}
}

func TestContractSpecChangeMakesDocStale(t *testing.T) {
	root := gitRepoWithDoc(t) // committed repo with a doc in sync (staleness_test.go)
	writeFile(t, root, "api/openapi.yaml", "openapi: 3.1.0\n")
	contractDoc(t, root, "svc", "provides:\n  - id: svc-api\n    spec: api/openapi.yaml\n")
	run(t, "sync", root)
	commitAll(t, root, "add contract with spec")

	writeFile(t, root, "api/openapi.yaml", "openapi: 3.1.1\n")
	commitAll(t, root, "change spec only")
	out, _ := checkOut(t, root)
	if !hasStaleFinding(out, "docs/architecture/modules/svc.md") {
		t.Errorf("a spec change after its doc should be stale, got:\n%s", out)
	}
}

func TestRepoConfigValidation(t *testing.T) {
	root := initRepo(t)
	writeFile(t, root, ".maat.yml", "repo: not-a-repo\n")
	_, stderr, code := run(t, "check", root)
	if code != 2 || !strings.Contains(stderr, "owner/name") {
		t.Errorf("expected config error (exit 2) for a bad repo, got exit %d: %s", code, stderr)
	}
}
