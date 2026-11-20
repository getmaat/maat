package maat

import (
	"strings"
	"testing"
)

// --------------------------------------------------------------------------- #
// cross-repo contracts via the lockfile (ADR 0014) — all offline
// --------------------------------------------------------------------------- #

const ledgerSourceCfg = "contracts:\n  sources:\n    acme/ledger:\n"

// lockWith returns a lockfile with one acme/ledger source.
func lockWith(sourceJSON string) string {
	return `{"schema": 1, "sources": {"acme/ledger": ` + sourceJSON + `}}`
}

// crossRepoRepo sets up a consumer of acme/ledger#events with the given
// .maat.yml, constraint and lockfile ("" = no lockfile).
func crossRepoRepo(t *testing.T, cfg, constraint, lock string) string {
	t.Helper()
	root := initRepo(t)
	writeFile(t, root, ".maat.yml", cfg)
	extra := "consumes:\n  - ref: acme/ledger#events\n"
	if constraint != "" {
		extra += "    version: \"" + constraint + "\"\n"
	}
	contractDoc(t, root, "client", extra)
	if lock != "" {
		writeFile(t, root, lockPath, lock)
	}
	return root
}

func expectFinding(t *testing.T, out string, code int, rule, want string) {
	t.Helper()
	if code != 1 || !strings.Contains(out, "["+rule+"]") || !strings.Contains(out, want) {
		t.Errorf("expected [%s] containing %q (exit 1), got exit %d:\n%s", rule, want, code, out)
	}
}

func TestCrossRepoResolvedFromLock(t *testing.T) {
	root := crossRepoRepo(t, ledgerSourceCfg, "~> 1.4",
		lockWith(`{"commit": "3f9c2e1aa", "manifest_schema": 1,
			"provides": [{"id": "events", "kind": "event", "version": "1.6.0"}]}`))
	if out, code := syncAndCheck(t, root); code != 0 {
		t.Errorf("a satisfied cross-repo ref should pass, got exit %d:\n%s", code, out)
	}
}

func TestCrossRepoVersionMismatch(t *testing.T) {
	root := crossRepoRepo(t, ledgerSourceCfg, "~> 1.4",
		lockWith(`{"commit": "3f9c2e1aa", "manifest_schema": 1,
			"provides": [{"id": "events", "version": "2.0.0"}]}`))
	out, code := syncAndCheck(t, root)
	expectFinding(t, out, code, "contract_version", "acme/ledger@3f9c2e1 provides version 2.0.0")
}

func TestCrossRepoProviderLacksContract(t *testing.T) {
	root := crossRepoRepo(t, ledgerSourceCfg, "",
		lockWith(`{"manifest_schema": 1, "provides": [], "missing": ["events"]}`))
	out, code := syncAndCheck(t, root)
	expectFinding(t, out, code, "contract_unresolved", "acme/ledger does not provide 'events'")
}

func TestCrossRepoMissingSource(t *testing.T) {
	root := crossRepoRepo(t, "", "", "")
	out, code := syncAndCheck(t, root)
	expectFinding(t, out, code, "contract_schema", "no contracts.sources entry for acme/ledger")
}

func TestCrossRepoLockDrift(t *testing.T) {
	cases := []struct {
		name, cfg, lock, want string
	}{
		{"no lockfile", ledgerSourceCfg, "", "missing — run `maat contracts update`"},
		{"unreadable", ledgerSourceCfg, "{not json", "unreadable"},
		{"unsupported schema", ledgerSourceCfg, `{"schema": 2, "sources": {}}`, "schema 2 is not supported"},
		{"source not locked", ledgerSourceCfg, `{"schema": 1, "sources": {}}`, "has no entry for acme/ledger"},
		{"ref added after update", ledgerSourceCfg,
			lockWith(`{"manifest_schema": 1, "provides": [{"id": "other"}]}`), "has no entry for acme/ledger#events"},
		{"settings changed", "contracts:\n  sources:\n    acme/ledger:\n      ref: release\n",
			lockWith(`{"manifest_schema": 1, "provides": [{"id": "events"}]}`), "different source settings"},
		{"source no longer consumed", ledgerSourceCfg,
			`{"schema": 1, "sources": {"acme/ledger": {"manifest_schema": 1, "provides": [{"id": "events"}]},
			"acme/old": {"manifest_schema": 1, "provides": []}}}`, "lists acme/old"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := crossRepoRepo(t, tc.cfg, "", tc.lock)
			out, code := syncAndCheck(t, root)
			expectFinding(t, out, code, "drift", tc.want)
			if !strings.Contains(out, lockPath) {
				t.Errorf("drift should point at %s:\n%s", lockPath, out)
			}
		})
	}
}

func TestLockWithoutCrossRepoRefsIsDrift(t *testing.T) {
	root := initRepo(t)
	writeFile(t, root, lockPath, `{"schema": 1, "sources": {}}`)
	out, code := syncAndCheck(t, root)
	expectFinding(t, out, code, "drift", "no cross-repo contracts are consumed")
}

func TestNoCrossRepoNoLockIsClean(t *testing.T) {
	root := initRepo(t)
	writeFile(t, root, ".maat.yml", ledgerSourceCfg) // configured but unused source
	if out, code := syncAndCheck(t, root); code != 0 {
		t.Errorf("an unused source without a lockfile should pass, got exit %d:\n%s", code, out)
	}
}

func TestSelfRefResolvesLocally(t *testing.T) {
	root := initRepo(t)
	writeFile(t, root, ".maat.yml", "repo: acme/billing\n")
	contractDoc(t, root, "svc", "provides:\n  - id: invoice-api\n    version: 2.1.0\n")
	contractDoc(t, root, "client", "consumes:\n  - ref: acme/billing#invoice-api\n    version: \"~> 2.1\"\n")
	if out, code := syncAndCheck(t, root); code != 0 {
		t.Errorf("a full ref to this repository should resolve locally, got exit %d:\n%s", code, out)
	}
}

func TestCrossRepoDriftSeverityFollowsDriftIsError(t *testing.T) {
	root := crossRepoRepo(t, ledgerSourceCfg+"check:\n  drift_is_error: false\n", "", "")
	out, code := syncAndCheck(t, root)
	if code != 0 || !strings.Contains(out, "[drift]") {
		t.Errorf("drift_is_error: false should make a missing lockfile a warning, got exit %d:\n%s", code, out)
	}
}

func TestContractSourcesValidation(t *testing.T) {
	cases := []struct {
		name, cfg, want string
	}{
		{"flow mapping", "contracts:\n  sources:\n    acme/ledger: {}\n", "instead of {}"},
		{"bad repo key", "contracts:\n  sources:\n    ledger:\n", "must be \"owner/name\""},
		{"unknown key", "contracts:\n  sources:\n    acme/ledger:\n      branch: main\n", "branch is not a known key"},
		{"url and path", "contracts:\n  sources:\n    acme/ledger:\n      url: https://x.example/{repo}\n      path: ../ledger\n", "not both"},
		{"plain http", "contracts:\n  sources:\n    acme/ledger:\n      url: http://x.example/{repo}\n", "https://"},
		{"numeric ref", "contracts:\n  sources:\n    acme/ledger:\n      ref: 1.10\n", "quote it"},
		{"unknown contracts key", "contracts:\n  index: x\n", "contracts.index is not a known key"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := initRepo(t)
			writeFile(t, root, ".maat.yml", tc.cfg)
			_, stderr, code := run(t, "check", root)
			if code != 2 || !strings.Contains(stderr, tc.want) {
				t.Errorf("expected config error (exit 2) containing %q, got exit %d: %s", tc.want, code, stderr)
			}
		})
	}
}
