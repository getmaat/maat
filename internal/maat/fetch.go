package maat

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Fetching provider manifests for `maat contracts update` (ADR 0014). This is
// the ONLY code in Ma'at that uses the network; sync and check never call it.
// It uses the standard library's HTTP client, which honors HTTPS_PROXY.

// httpClient is a package variable so tests can substitute a client that
// trusts their TLS test server.
var httpClient = &http.Client{Timeout: 30 * time.Second}

// maxManifestBytes bounds how much of a response is read.
const maxManifestBytes = 5 << 20

// remoteManifest is the subset of a provider's .maat/manifest.json that
// update needs.
type remoteManifest struct {
	Schema   int        `json:"schema"`
	Provides []Contract `json:"provides"`
}

// fetchedManifest is a provider manifest plus the commit it was read at, when
// the source can tell (GitHub only).
type fetchedManifest struct {
	manifest remoteManifest
	commit   string
}

// fetchManifest reads repo's manifest from its configured source. root is
// this repository's root, against which relative `path` sources resolve.
func fetchManifest(root, repo string, src contractSource) (fetchedManifest, error) {
	var (
		data   []byte
		commit string
		err    error
	)
	switch {
	case src.Path != "":
		dir := src.Path
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(root, dir)
		}
		data, err = os.ReadFile(filepath.Join(dir, manifestPath))
		if err != nil {
			return fetchedManifest{}, fmt.Errorf("cannot read %s from path %s: %w", manifestPath, src.Path, err)
		}
	case src.URL != "":
		data, err = fetchFromURL(repo, src)
	default:
		data, commit, err = fetchFromGitHub(repo, src.Ref)
	}
	if err != nil {
		return fetchedManifest{}, err
	}
	var m remoteManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return fetchedManifest{}, fmt.Errorf("the manifest is not valid JSON: %w", err)
	}
	switch {
	case m.Schema == 0:
		return fetchedManifest{}, fmt.Errorf("the response is not a Ma'at manifest (no \"schema\" field)")
	case m.Schema > manifestSchema:
		return fetchedManifest{}, fmt.Errorf("the manifest uses schema %d, newer than this maat supports (%d) — upgrade maat",
			m.Schema, manifestSchema)
	}
	return fetchedManifest{manifest: m, commit: commit}, nil
}

// githubAPIBase is the GitHub REST API root. GITHUB_API_URL is set by GitHub
// Actions (including on GitHub Enterprise Server); tests use it too.
func githubAPIBase() string {
	if v := strings.TrimSpace(os.Getenv("GITHUB_API_URL")); v != "" {
		return strings.TrimRight(v, "/")
	}
	return "https://api.github.com"
}

func githubToken() string {
	for _, k := range []string{"GITHUB_TOKEN", "GH_TOKEN"} {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return ""
}

// fetchFromGitHub resolves ref (default branch when empty) to a commit first,
// then reads the manifest at exactly that commit, so the two always agree.
func fetchFromGitHub(repo, ref string) (data []byte, commit string, err error) {
	if ref == "" {
		ref = "HEAD"
	}
	base := githubAPIBase() + "/repos/" + repo
	token := githubToken()

	sha, status, err := httpGet(base+"/commits/"+escapePath(ref), "application/vnd.github.sha", token)
	if err != nil {
		return nil, "", fmt.Errorf("cannot reach GitHub: %w", err)
	}
	if status != http.StatusOK {
		return nil, "", githubError(repo, ref, status, token != "")
	}
	commit = strings.TrimSpace(string(sha))

	data, status, err = httpGet(base+"/contents/"+manifestPath+"?ref="+url.QueryEscape(commit),
		"application/vnd.github.raw+json", token)
	if err != nil {
		return nil, "", fmt.Errorf("cannot reach GitHub: %w", err)
	}
	if status == http.StatusNotFound {
		return nil, "", fmt.Errorf("%s has no %s at %s — run `maat sync` there (Ma'at 0.8 or later) and commit it",
			repo, manifestPath, ref)
	}
	if status != http.StatusOK {
		return nil, "", githubError(repo, ref, status, token != "")
	}
	return data, commit, nil
}

func githubError(repo, ref string, status int, hasToken bool) error {
	switch status {
	case http.StatusNotFound:
		if hasToken {
			return fmt.Errorf("%s (ref %s) not found, or the token cannot read it", repo, ref)
		}
		return fmt.Errorf("%s (ref %s) not found — if it is private, set GITHUB_TOKEN or GH_TOKEN", repo, ref)
	case http.StatusUnauthorized:
		return fmt.Errorf("GitHub rejected the token for %s — check GITHUB_TOKEN / GH_TOKEN", repo)
	case http.StatusForbidden, http.StatusTooManyRequests:
		return fmt.Errorf("GitHub refused access to %s (HTTP %d: no permission, or rate-limited — a token raises the limit)",
			repo, status)
	default:
		return fmt.Errorf("GitHub returned HTTP %d for %s", status, repo)
	}
}

// fetchFromURL fills a source's URL template and reads the manifest from it.
// The token in MAAT_TOKEN, if any, is sent as a bearer token — and only to
// this URL; GitHub tokens are never sent to other hosts.
func fetchFromURL(repo string, src contractSource) ([]byte, error) {
	if strings.Contains(src.URL, "{ref}") && src.Ref == "" {
		return nil, fmt.Errorf("the url template uses {ref}, but no ref is set for %s", repo)
	}
	target := strings.NewReplacer("{repo}", repo, "{ref}", src.Ref).Replace(src.URL)
	data, status, err := httpGet(target, "application/json", strings.TrimSpace(os.Getenv("MAAT_TOKEN")))
	if err != nil {
		return nil, fmt.Errorf("cannot reach %s: %w", hostOf(target), err)
	}
	switch status {
	case http.StatusOK:
		return data, nil
	case http.StatusNotFound:
		return nil, fmt.Errorf("%s returned 404 for %s — check the url template and ref", hostOf(target), repo)
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, fmt.Errorf("%s refused access (HTTP %d) — set MAAT_TOKEN", hostOf(target), status)
	default:
		return nil, fmt.Errorf("%s returned HTTP %d for %s", hostOf(target), status, repo)
	}
}

// httpGet performs a GET and returns the (size-limited) body and status.
// Go's client drops the Authorization header on redirects to another host.
func httpGet(target, accept, token string) ([]byte, int, error) {
	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", accept)
	req.Header.Set("User-Agent", "maat/"+Version())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }() // read-only; a close error changes nothing
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxManifestBytes+1))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if len(body) > maxManifestBytes {
		return nil, resp.StatusCode, fmt.Errorf("response larger than %d bytes", maxManifestBytes)
	}
	return body, resp.StatusCode, nil
}

// escapePath escapes each segment of a ref but keeps "/" so branch names like
// "release/1.x" reach the API as-is.
func escapePath(ref string) string {
	parts := strings.Split(ref, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return strings.Join(parts, "/")
}

func hostOf(target string) string {
	if u, err := url.Parse(target); err == nil && u.Host != "" {
		return u.Host
	}
	return "the source"
}
