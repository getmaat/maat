package maat

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// --------------------------------------------------------------------------- #
// maat contracts update (ADR 0014 §2) — against local fake servers only
// --------------------------------------------------------------------------- #

// fakeGitHub serves /repos/{repo}/commits/{ref} and
// /repos/{repo}/contents/.maat/manifest.json for the configured repos, and
// records the Authorization header of every request.
type fakeGitHub struct {
	mu        sync.Mutex
	commit    map[string]string // repo -> sha
	manifests map[string]string // repo -> manifest JSON
	auth      []string
}

func newFakeGitHub(t *testing.T) *fakeGitHub {
	t.Helper()
	f := &fakeGitHub{commit: map[string]string{}, manifests: map[string]string{}}
	srv := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(srv.Close)
	t.Setenv("GITHUB_API_URL", srv.URL)
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("MAAT_TOKEN", "")
	return f
}

func (f *fakeGitHub) set(repo, sha, manifest string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.commit[repo], f.manifests[repo] = sha, manifest
}

func (f *fakeGitHub) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.auth = append(f.auth, r.Header.Get("Authorization"))
	path := strings.TrimPrefix(r.URL.Path, "/repos/")
	for repo, sha := range f.commit {
		switch {
		case strings.HasPrefix(path, repo+"/commits/"):
			_, _ = w.Write([]byte(sha))
			return
		case path == repo+"/contents/"+manifestPath:
			if r.URL.Query().Get("ref") != sha {
				http.Error(w, "manifest must be read at the resolved commit", http.StatusBadRequest)
				return
			}
			if f.manifests[repo] == "" {
				http.NotFound(w, r)
				return
			}
			_, _ = w.Write([]byte(f.manifests[repo]))
			return
		}
	}
	http.NotFound(w, r)
}

func ledgerManifest(version string) string {
	return `{"schema": 1, "repo": "acme/ledger", "provides": [
		{"id": "events", "kind": "event", "version": "` + version + `", "doc": "docs/events.md"},
		{"id": "unused", "version": "9.9.9", "doc": "docs/unused.md"}], "consumes": []}`
}

// consumerRepo is a repo consuming acme/ledger#events (and optionally more).
func consumerRepo(t *testing.T, cfg, consumes string) string {
	t.Helper()
	root := initRepo(t)
	writeFile(t, root, ".maat.yml", cfg)
	contractDoc(t, root, "client", consumes)
	if _, _, code := run(t, "sync", root); code != 0 {
		t.Fatalf("sync exited %d", code)
	}
	return root
}

const consumesEvents = "consumes:\n  - ref: acme/ledger#events\n    version: \"~> 1.4\"\n"

func update(t *testing.T, root string, extra ...string) (string, string, int) {
	t.Helper()
	return run(t, append([]string{"contracts", "update", root}, extra...)...)
}

func TestUpdateWritesLockAndCheckPasses(t *testing.T) {
	gh := newFakeGitHub(t)
	gh.set("acme/ledger", "3f9c2e1aaaaa", ledgerManifest("1.6.0"))
	root := consumerRepo(t, ledgerSourceCfg, consumesEvents)

	out, stderr, code := update(t, root)
	if code != 0 {
		t.Fatalf("update exited %d:\n%s%s", code, out, stderr)
	}
	lock, _, err := readLock(root)
	if err != nil || lock == nil {
		t.Fatalf("lockfile not written: %v", err)
	}
	src := lock.Sources["acme/ledger"]
	if src.Commit != "3f9c2e1aaaaa" || len(src.Provides) != 1 || src.Provides[0].Version != "1.6.0" ||
		src.Provides[0].Doc != "docs/events.md" {
		t.Errorf("unexpected entry (only the consumed contract should be locked): %+v", src)
	}
	if out, code := checkOut(t, root); code != 0 {
		t.Errorf("check should pass after update, got exit %d:\n%s", code, out)
	}
}

func TestUpdateRecordsMissingAndCheckReportsIt(t *testing.T) {
	gh := newFakeGitHub(t)
	gh.set("acme/ledger", "aaa", ledgerManifest("1.6.0"))
	root := consumerRepo(t, ledgerSourceCfg, consumesEvents+"  - ref: acme/ledger#gone\n")
	if _, _, code := update(t, root); code != 0 {
		t.Fatal("update failed")
	}
	lock, _, _ := readLock(root)
	if got := lock.Sources["acme/ledger"].Missing; len(got) != 1 || got[0] != "gone" {
		t.Errorf("missing = %v, want [gone]", got)
	}
	out, code := checkOut(t, root)
	expectFinding(t, out, code, "contract_unresolved", "acme/ledger@aaa does not provide 'gone'")
}

func TestUpdateCheckDetectsVersionChange(t *testing.T) {
	gh := newFakeGitHub(t)
	gh.set("acme/ledger", "aaa", ledgerManifest("1.6.0"))
	root := consumerRepo(t, ledgerSourceCfg, consumesEvents)
	update(t, root)

	if out, _, code := update(t, root, "--check"); code != 0 || !strings.Contains(out, "up to date") {
		t.Fatalf("expected up to date, got exit %d:\n%s", code, out)
	}
	gh.set("acme/ledger", "bbb", ledgerManifest("2.0.0"))
	out, _, code := update(t, root, "--check")
	if code != 1 || !strings.Contains(out, "acme/ledger#events: 1.6.0 → 2.0.0") {
		t.Errorf("expected the version change to be reported (exit 1), got exit %d:\n%s", code, out)
	}
	if lock, _, _ := readLock(root); lock.Sources["acme/ledger"].Provides[0].Version != "1.6.0" {
		t.Error("--check must not write the lockfile")
	}
	// After updating, check reports the incompatibility.
	update(t, root)
	out, code = checkOut(t, root)
	expectFinding(t, out, code, "contract_version", "provides version 2.0.0")
}

func TestUpdateIgnoresCommitOnlyChanges(t *testing.T) {
	gh := newFakeGitHub(t)
	gh.set("acme/ledger", "aaa", ledgerManifest("1.6.0"))
	root := consumerRepo(t, ledgerSourceCfg, consumesEvents)
	update(t, root)
	before := readFile(t, root, lockPath)

	// The provider committed again and changed a doc path, but the consumed
	// contract's version did not change: no churn, no failing --check.
	gh.set("acme/ledger", "ccc", strings.Replace(ledgerManifest("1.6.0"), "docs/events.md", "docs/moved.md", 1))
	if out, _, code := update(t, root, "--check"); code != 0 {
		t.Errorf("a commit-only change must not fail --check, got exit %d:\n%s", code, out)
	}
	update(t, root)
	if after := readFile(t, root, lockPath); after != before {
		t.Errorf("update must not rewrite the lockfile for a commit-only change:\nbefore %s\nafter %s", before, after)
	}
}

func TestUpdateTokenHandling(t *testing.T) {
	gh := newFakeGitHub(t)
	gh.set("acme/ledger", "aaa", ledgerManifest("1.6.0"))
	root := consumerRepo(t, ledgerSourceCfg, consumesEvents)

	update(t, root)
	for _, a := range gh.auth {
		if a != "" {
			t.Fatalf("no token is configured, but Authorization was sent: %q", a)
		}
	}
	gh.auth = nil
	t.Setenv("GH_TOKEN", "secret-token")
	out, stderr, _ := update(t, root)
	if len(gh.auth) == 0 || gh.auth[0] != "Bearer secret-token" {
		t.Errorf("GH_TOKEN should be sent as a bearer token, got %v", gh.auth)
	}
	if strings.Contains(out+stderr, "secret-token") {
		t.Error("the token must never be printed")
	}
}

func TestUpdateFailuresLeaveLockUntouched(t *testing.T) {
	gh := newFakeGitHub(t)
	gh.set("acme/ledger", "aaa", ledgerManifest("1.6.0"))
	root := consumerRepo(t, ledgerSourceCfg+"    acme/other:\n",
		consumesEvents+"  - ref: acme/other#api\n")
	writeFile(t, root, lockPath, `{"schema": 1, "sources": {}}`)

	// acme/other is unknown to the fake server: 404 without a token.
	out, stderr, code := update(t, root)
	if code != 1 || !strings.Contains(stderr, "acme/other") || !strings.Contains(stderr, "set GITHUB_TOKEN or GH_TOKEN") {
		t.Errorf("expected a helpful failure (exit 1), got exit %d:\n%s%s", code, out, stderr)
	}
	if got := readFile(t, root, lockPath); got != `{"schema": 1, "sources": {}}` {
		t.Errorf("a failed update must not change the lockfile, got %s", got)
	}
}

func TestUpdateErrors(t *testing.T) {
	cases := []struct {
		name, cfg, manifest, want string
	}{
		{"no source", "", ledgerManifest("1.0.0"), "no contracts.sources entry"},
		{"no manifest in repo", ledgerSourceCfg, "", "has no .maat/manifest.json"},
		{"not a manifest", ledgerSourceCfg, `{"hello": "world"}`, "not a Ma'at manifest"},
		{"newer schema", ledgerSourceCfg, `{"schema": 2, "provides": []}`, "upgrade maat"},
		{"invalid json", ledgerSourceCfg, `{nope`, "not valid JSON"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gh := newFakeGitHub(t)
			gh.set("acme/ledger", "aaa", tc.manifest)
			root := consumerRepo(t, tc.cfg, consumesEvents)
			_, stderr, code := update(t, root)
			if code != 1 || !strings.Contains(stderr, tc.want) {
				t.Errorf("expected exit 1 with %q, got exit %d:\n%s", tc.want, code, stderr)
			}
			if _, err := os.Stat(filepath.Join(root, lockPath)); err == nil {
				t.Error("no lockfile may be written on failure")
			}
		})
	}
}

func TestUpdateFromURLTemplate(t *testing.T) {
	newFakeGitHub(t) // resets token env vars
	var gotPath, gotAuth string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
		_, _ = w.Write([]byte(ledgerManifest("1.5.0")))
	}))
	t.Cleanup(srv.Close)
	orig := httpClient
	httpClient = srv.Client()
	t.Cleanup(func() { httpClient = orig })
	t.Setenv("GITHUB_TOKEN", "github-secret")
	t.Setenv("MAAT_TOKEN", "maat-secret")

	cfg := "contracts:\n  sources:\n    acme/ledger:\n      url: " + srv.URL + "/raw/{repo}/{ref}/manifest.json\n      ref: main\n"
	root := consumerRepo(t, cfg, consumesEvents)
	if out, stderr, code := update(t, root); code != 0 {
		t.Fatalf("update exited %d:\n%s%s", code, out, stderr)
	}
	if gotPath != "/raw/acme/ledger/main/manifest.json" {
		t.Errorf("template not filled: %s", gotPath)
	}
	if gotAuth != "Bearer maat-secret" {
		t.Errorf("url sources must send MAAT_TOKEN only, got %q", gotAuth)
	}
	lock, _, _ := readLock(root)
	if src := lock.Sources["acme/ledger"]; src.URL == "" || src.Ref != "main" || src.Commit != "" {
		t.Errorf("unexpected url-source entry: %+v", src)
	}
	if out, code := checkOut(t, root); code != 0 {
		t.Errorf("check should pass, got exit %d:\n%s", code, out)
	}
}

func TestUpdateURLTemplateNeedsRef(t *testing.T) {
	newFakeGitHub(t)
	root := consumerRepo(t, "contracts:\n  sources:\n    acme/ledger:\n      url: https://x.example/{repo}/{ref}\n", consumesEvents)
	_, stderr, code := update(t, root)
	if code != 1 || !strings.Contains(stderr, "no ref is set") {
		t.Errorf("expected a missing-ref error, got exit %d:\n%s", code, stderr)
	}
}

func TestUpdateFromLocalPath(t *testing.T) {
	newFakeGitHub(t)
	parent := t.TempDir()
	writeFile(t, parent, "ledger/"+manifestPath, ledgerManifest("1.4.2"))
	root := filepath.Join(parent, "consumer")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	run(t, "init", root, "--name", "P", "--summary", "S.")
	writeFile(t, root, ".maat.yml", "contracts:\n  sources:\n    acme/ledger:\n      path: ../ledger\n")
	contractDoc(t, root, "client", consumesEvents)
	run(t, "sync", root)

	if out, stderr, code := update(t, root); code != 0 {
		t.Fatalf("update exited %d:\n%s%s", code, out, stderr)
	}
	if out, code := checkOut(t, root); code != 0 {
		t.Errorf("check should pass, got exit %d:\n%s", code, out)
	}
}

func TestUpdateRemovesLockWhenNothingConsumed(t *testing.T) {
	newFakeGitHub(t)
	root := initRepo(t)
	writeFile(t, root, lockPath, `{"schema": 1, "sources": {}}`)
	out, _, code := update(t, root)
	if code != 0 || !strings.Contains(out, "remove") {
		t.Fatalf("expected removal, got exit %d:\n%s", code, out)
	}
	if _, err := os.Stat(filepath.Join(root, lockPath)); err == nil {
		t.Error("the lockfile should be removed")
	}
	if out, _, code := update(t, root, "--check"); code != 0 {
		t.Errorf("--check with nothing consumed and no lockfile should pass, got %d:\n%s", code, out)
	}
}

func TestContractsCommandUsage(t *testing.T) {
	for _, args := range [][]string{{"contracts"}, {"contracts", "fetch"}, {"contracts", "update", "--strict"}} {
		if _, _, code := run(t, args...); code != 2 {
			t.Errorf("%v: expected usage error (exit 2), got %d", args, code)
		}
	}
}

func TestUpdateMultipleSources(t *testing.T) {
	gh := newFakeGitHub(t)
	gh.set("acme/ledger", "aaa", ledgerManifest("1.6.0"))
	gh.set("acme/billing", "bbb", `{"schema": 1, "provides": [{"id": "invoice-api", "version": "2.1.0"}]}`)
	root := consumerRepo(t, ledgerSourceCfg+"    acme/billing:\n",
		consumesEvents+"  - ref: acme/billing#invoice-api\n    version: \"~> 2.0\"\n")

	out, stderr, code := update(t, root)
	if code != 0 {
		t.Fatalf("update exited %d:\n%s%s", code, out, stderr)
	}
	lock, _, _ := readLock(root)
	if len(lock.Sources) != 2 || lock.Sources["acme/billing"].Commit != "bbb" || lock.Sources["acme/ledger"].Commit != "aaa" {
		t.Errorf("both sources should be locked: %+v", lock.Sources)
	}
	// encoding/json sorts map keys, so the file is deterministic.
	text := readFile(t, root, lockPath)
	if strings.Index(text, "acme/billing") > strings.Index(text, "acme/ledger") {
		t.Errorf("sources should be sorted:\n%s", text)
	}
	if out, code := checkOut(t, root); code != 0 {
		t.Errorf("check should pass, got exit %d:\n%s", code, out)
	}
}
