package maat

import (
	"reflect"
	"strings"
	"testing"
)

// The YAML subset parser had no tests of its own; these pin down the forms it
// supports and, above all, that it never drops input silently.

func TestYAMLParseSupportedForms(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want any
	}{
		{
			name: "scalars and coercion",
			in:   "s: hello\ni: 3\nf: 1.5\nb: true\nn: null\nq: \"a: b\"\n",
			want: map[string]any{"s": "hello", "i": int64(3), "f": 1.5, "b": true, "n": nil, "q": "a: b"},
		},
		{
			name: "flow list",
			in:   "adapters: [claude, cursor]\n",
			want: map[string]any{"adapters": []any{"claude", "cursor"}},
		},
		{
			name: "indented block sequence",
			in:   "related_code:\n  - a.go\n  - b.go\n",
			want: map[string]any{"related_code": []any{"a.go", "b.go"}},
		},
		{
			name: "block sequence at the same indent as its key",
			in:   "related_code:\n- a.go\n- b.go\ntitle: T\n",
			want: map[string]any{"related_code": []any{"a.go", "b.go"}, "title": "T"},
		},
		{
			name: "compact list of mappings",
			in: "provides:\n  - id: invoice-api\n    kind: http-api\n    version: 2.1.0\n" +
				"  - id: ledger\nconsumes:\n  - ref: org/billing#invoice-api\n",
			want: map[string]any{
				"provides": []any{
					map[string]any{"id": "invoice-api", "kind": "http-api", "version": "2.1.0"},
					map[string]any{"id": "ledger"},
				},
				"consumes": []any{map[string]any{"ref": "org/billing#invoice-api"}},
			},
		},
		{
			name: "compact mapping with nested values",
			in:   "items:\n  - name: a\n    tags:\n      - x\n      - y\n    meta:\n      k: v\n",
			want: map[string]any{"items": []any{map[string]any{
				"name": "a", "tags": []any{"x", "y"}, "meta": map[string]any{"k": "v"},
			}}},
		},
		{
			name: "compact mapping item with an empty first value",
			in:   "items:\n  - name:\n    other: 1\n",
			want: map[string]any{"items": []any{map[string]any{"name": nil, "other": int64(1)}}},
		},
		{
			name: "dash on its own line followed by a mapping",
			in:   "items:\n  -\n    name: a\n",
			want: map[string]any{"items": []any{map[string]any{"name": "a"}}},
		},
		{
			name: "URLs and quoted colons stay scalars",
			in:   "links:\n  - https://example.com/a\n  - \"key: value\"\n",
			want: map[string]any{"links": []any{"https://example.com/a", "key: value"}},
		},
		{
			name: "template placeholders still parse as strings",
			in:   "title: {{PROJECT}} Documentation\n",
			want: map[string]any{"title": "{{PROJECT}} Documentation"},
		},
		{
			name: "comments and blank lines",
			in:   "# header\n\na: 1 # trailing\nb: \"x # not a comment\"\n",
			want: map[string]any{"a": int64(1), "b": "x # not a comment"},
		},
		{
			name: "empty document",
			in:   "\n# only a comment\n",
			want: map[string]any{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := yamlParse(tc.in)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got  %#v\nwant %#v", got, tc.want)
			}
		})
	}
}

// Input the parser cannot place must be an error with a line number, never
// dropped. Each of these silently lost data before.
func TestYAMLParseNeverDropsInputSilently(t *testing.T) {
	cases := []struct {
		name, in, wantLine string
	}{
		{"over-indented continuation", "a: 1\n    b: 2\n", "line 2"},
		{"inconsistent nested indent", "a:\n    b: 1\n  c: 2\n", "line 3"},
		{"block scalar", "summary: |\n  first line\n  second line\n", "line 2"},
		{"misaligned key in a compact mapping", "items:\n  - id: a\n      kind: b\n", "line 3"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := yamlParse(tc.in)
			if err == nil {
				t.Fatalf("expected an error, got %#v", got)
			}
			if _, ok := err.(*YAMLError); !ok {
				t.Errorf("expected *YAMLError, got %T", err)
			}
			if !strings.Contains(err.Error(), tc.wantLine) {
				t.Errorf("error should name %s: %v", tc.wantLine, err)
			}
		})
	}
}

func TestYAMLParseTabsRejected(t *testing.T) {
	if _, err := yamlParse("a:\n\t- x\n"); err == nil {
		t.Error("tab indentation must be rejected")
	}
}

// End to end: malformed front matter surfaces through check as a finding
// instead of a doc whose metadata quietly went missing.
func TestCheckReportsUnparseableFrontMatter(t *testing.T) {
	root := initRepo(t)
	writeFile(t, root, "docs/guides/bad.md",
		"---\ntitle: Bad\nstatus: current\nsummary: |\n  multi\n  line\n---\n# Bad\n")
	out, stderr, code := run(t, "check", root)
	if code == 0 {
		t.Fatalf("expected a failure for unparseable front matter, got exit 0:\n%s", out)
	}
	if !strings.Contains(out+stderr, "line") {
		t.Errorf("expected the parse error to be reported, got:\nstdout: %s\nstderr: %s", out, stderr)
	}
}
