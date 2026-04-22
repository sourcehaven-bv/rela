package lua

import (
	"bytes"
	"strings"
	"testing"
)

// fakeCatalog is a minimal RouteCatalog for tests — an allowlist of paths
// that Has reports true for. Keeps the Lua binding tests isolated from the
// frontendroutes package.
type fakeCatalog struct {
	known map[string]bool
}

func (f fakeCatalog) Has(path string) bool { return f.known[path] }

func newCatalog(paths ...string) fakeCatalog {
	m := make(map[string]bool, len(paths))
	for _, p := range paths {
		m[p] = true
	}
	return fakeCatalog{known: m}
}

// newURLWriter builds a writer runtime with a supplied route catalog.
func newURLWriter(t *testing.T, cat RouteCatalog) *Runtime {
	t.Helper()
	ws := newMockWorkspace(t)
	var buf bytes.Buffer
	r := NewWriter(ws.services("/tmp"), &buf, WithRouteCatalog(cat))
	t.Cleanup(r.Close)
	return r
}

// evalString runs `return <expr>` and returns the resulting Lua string.
// Fails the test on eval error or non-string result.
func evalString(t *testing.T, r *Runtime, expr string) string {
	t.Helper()
	if err := r.RunString("result = " + expr); err != nil {
		t.Fatalf("eval %q: %v", expr, err)
	}
	v := r.L.GetGlobal("result")
	s, ok := v.(interface{ String() string })
	if !ok {
		t.Fatalf("expected string result for %q, got %T", expr, v)
	}
	return s.String()
}

func TestURL_notRegisteredWithoutOption(t *testing.T) {
	ws := newMockWorkspace(t)
	var buf bytes.Buffer
	r := NewWriter(ws.services("/tmp"), &buf)
	defer r.Close()
	err := r.RunString(`return rela.url("/dashboard")`)
	if err == nil {
		t.Fatal("expected error — rela.url should not be registered without WithRouteCatalog")
	}
	// Absent binding → Lua raises "attempt to call a nil value" (older gopher-lua)
	// or "attempt to call a non-function object" (current). Either indicates the
	// binding isn't registered.
	msg := err.Error()
	if !strings.Contains(msg, "nil value") && !strings.Contains(msg, "non-function object") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestURL_happyPath(t *testing.T) {
	cat := newCatalog("/form/full_ticket", "/form/full_ticket/TKT-001", "/dashboard")
	r := newURLWriter(t, cat)

	cases := []struct {
		name string
		code string
		want string
	}{
		{
			name: "path only",
			code: `rela.url("/dashboard")`,
			want: "/dashboard",
		},
		{
			name: "path with params",
			code: `rela.url("/form/full_ticket", {["prop.status"]="open", q="a b&c"})`,
			want: "/form/full_ticket?prop.status=open&q=a+b%26c",
		},
		{
			name: "path with edit form",
			code: `rela.url("/form/full_ticket/TKT-001")`,
			want: "/form/full_ticket/TKT-001",
		},
		{
			name: "existing query preserved and merged",
			code: `rela.url("/form/full_ticket?x=1", {y="2"})`,
			want: "/form/full_ticket?x=1&y=2",
		},
		{
			name: "existing query overridden by param",
			code: `rela.url("/form/full_ticket?x=old", {x="new"})`,
			want: "/form/full_ticket?x=new",
		},
		{
			name: "fragment preserved",
			code: `rela.url("/form/full_ticket/TKT-001#section", {y="2"})`,
			want: "/form/full_ticket/TKT-001?y=2#section",
		},
		{
			name: "empty params table leaves path unchanged",
			code: `rela.url("/dashboard", {})`,
			want: "/dashboard",
		},
		{
			name: "number value stringified",
			code: `rela.url("/form/full_ticket", {page=3})`,
			want: "/form/full_ticket?page=3",
		},
		{
			name: "bool value stringified",
			code: `rela.url("/form/full_ticket", {draft=true})`,
			want: "/form/full_ticket?draft=true",
		},
		{
			name: "deterministic key order",
			code: `rela.url("/form/full_ticket", {b="2", a="1", c="3"})`,
			want: "/form/full_ticket?a=1&b=2&c=3",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := evalString(t, r, tc.code)
			if got != tc.want {
				t.Errorf("rela.url returned %q, want %q", got, tc.want)
			}
		})
	}
}

func TestURL_unknownPathRaises(t *testing.T) {
	r := newURLWriter(t, newCatalog("/dashboard"))
	err := r.RunString(`rela.url("/nope/foo")`)
	if err == nil {
		t.Fatal("expected error for unknown path")
	}
	if !strings.Contains(err.Error(), "unknown frontend route: /nope/foo") {
		t.Fatalf("error should name the unknown path, got: %v", err)
	}
}

func TestURL_typeErrors(t *testing.T) {
	r := newURLWriter(t, newCatalog("/x"))

	cases := []struct {
		name    string
		code    string
		wantSub string
	}{
		{
			name:    "non-table params arg",
			code:    `rela.url("/x", "not a table")`,
			wantSub: "table",
		},
		{
			name:    "function value",
			code:    `rela.url("/x", {a=function() end})`,
			wantSub: "param \"a\": value must be string, number, or boolean",
		},
		{
			name:    "nil value",
			code:    `rela.url("/x", {a=nil, b="ok"})`,
			wantSub: "", // nil values are dropped by Lua table semantics — no error
		},
		{
			name:    "key with &",
			code:    `rela.url("/x", {["a&b"]="1"})`,
			wantSub: "forbidden characters",
		},
		{
			name:    "key with =",
			code:    `rela.url("/x", {["a=b"]="1"})`,
			wantSub: "forbidden characters",
		},
		{
			name:    "key with whitespace",
			code:    `rela.url("/x", {["a b"]="1"})`,
			wantSub: "forbidden characters",
		},
		{
			name:    "empty key",
			code:    `rela.url("/x", {[""]="1"})`,
			wantSub: "empty",
		},
		{
			name:    "reserved key return_to",
			code:    `rela.url("/x", {return_to="/evil"})`,
			wantSub: "reserved",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := r.RunString(tc.code)
			if tc.wantSub == "" {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantSub)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("error %v does not contain %q", err, tc.wantSub)
			}
		})
	}
}

func TestURL_emptyQueryNoTrailingQuestion(t *testing.T) {
	r := newURLWriter(t, newCatalog("/x"))
	got := evalString(t, r, `rela.url("/x")`)
	if got != "/x" {
		t.Errorf("got %q, want %q", got, "/x")
	}
	got = evalString(t, r, `rela.url("/x", {})`)
	if got != "/x" {
		t.Errorf("got %q, want %q", got, "/x")
	}
}
