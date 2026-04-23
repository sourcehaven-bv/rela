package lua

import (
	"bytes"
	"strings"
	"testing"
)

// newURLWriter builds a writer runtime wired with the rela.url bindings.
func newURLWriter(t *testing.T) *Runtime {
	t.Helper()
	ws := newMockWorkspace(t)
	var buf bytes.Buffer
	r := NewWriter(ws.services("/tmp"), &buf)
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

func TestURLFormEdit(t *testing.T) {
	r := newURLWriter(t)
	got := evalString(t, r, `rela.url.form_edit("full_ticket", {id="TKT-001", type="ticket"})`)
	want := "/form/full_ticket/TKT-001"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestURLFormCreate(t *testing.T) {
	r := newURLWriter(t)
	cases := []struct {
		name string
		code string
		want string
	}{
		{
			name: "bare",
			code: `rela.url.form_create("full_ticket")`,
			want: "/form/full_ticket",
		},
		{
			name: "with relations",
			code: `rela.url.form_create("full_ticket", {relations = {parent = "TKT-PARENT", assignee = "actor-me"}})`,
			want: "/form/full_ticket?rel.assignee=actor-me&rel.parent=TKT-PARENT",
		},
		{
			name: "relation name with dash",
			code: `rela.url.form_create("full_ticket", {relations = {["belongs-to"] = "CAT-1"}})`,
			want: "/form/full_ticket?rel.belongs-to=CAT-1",
		},
		{
			name: "with properties",
			code: `rela.url.form_create("full_ticket", {properties = {status = "open", priority = "high"}})`,
			want: "/form/full_ticket?prop.priority=high&prop.status=open",
		},
		{
			name: "with relations + properties + query",
			code: `rela.url.form_create("full_ticket", {
                relations = {parent = "TKT-1"},
                properties = {status = "open"},
                query = {source = "doc"},
            })`,
			want: "/form/full_ticket?prop.status=open&rel.parent=TKT-1&source=doc",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := evalString(t, r, tc.code)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// form_edit takes an entity table and extracts the id — any other keys
// (including "query") are ignored. Authors wanting extra query on an
// edit URL have no supported shape today; the return_to rewriter is
// the usual source of form-link query, and extra ad-hoc params on
// edit URLs have no use case.
func TestURLFormEdit_ignoresExtraTableKeys(t *testing.T) {
	r := newURLWriter(t)
	got := evalString(t, r,
		`rela.url.form_edit("full_ticket", {id="TKT-001", type="ticket", source="doc"})`)
	want := "/form/full_ticket/TKT-001"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestURLForm_errors(t *testing.T) {
	r := newURLWriter(t)
	cases := []struct {
		name    string
		code    string
		wantSub string
	}{
		{
			name:    "form_edit empty name",
			code:    `rela.url.form_edit("", {id="x"})`,
			wantSub: "cannot be empty",
		},
		{
			name:    "form_edit entity without id",
			code:    `rela.url.form_edit("full_ticket", {type="ticket"})`,
			wantSub: "id",
		},
		{
			name:    "form_create empty name",
			code:    `rela.url.form_create("")`,
			wantSub: "cannot be empty",
		},
		{
			name:    "form_create relation key with prefix",
			code:    `rela.url.form_create("full_ticket", {relations = {["rel.parent"] = "TKT-1"}})`,
			wantSub: "forbidden characters",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := r.RunString(tc.code)
			if err == nil {
				t.Fatalf("expected error containing %q", tc.wantSub)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("error %v does not contain %q", err, tc.wantSub)
			}
		})
	}
}

func TestURLDetail(t *testing.T) {
	r := newURLWriter(t)
	got := evalString(t, r, `rela.url.detail({id="TKT-001", type="ticket"})`)
	want := "/entity/ticket/TKT-001"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestURLDetail_requiresFields(t *testing.T) {
	r := newURLWriter(t)
	cases := []string{
		`rela.url.detail({type="ticket"})`,          // no id
		`rela.url.detail({id="TKT-001"})`,           // no type
		`rela.url.detail({id=123, type="ticket"})`,  // id not string
		`rela.url.detail({id="TKT-001", type=nil})`, // type nil
	}
	for _, code := range cases {
		t.Run(code, func(t *testing.T) {
			err := r.RunString(code)
			if err == nil {
				t.Fatalf("expected error for %q", code)
			}
		})
	}
}

func TestURLList(t *testing.T) {
	r := newURLWriter(t)
	cases := []struct{ code, want string }{
		{`rela.url.list("all_tasks")`, "/list/all_tasks"},
		{`rela.url.list("all_tasks", {status = "open"})`, "/list/all_tasks?status=open"},
	}
	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			got := evalString(t, r, tc.code)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestURLView(t *testing.T) {
	r := newURLWriter(t)
	got := evalString(t, r, `rela.url.view("timeline", {id="TKT-001", type="ticket"})`)
	want := "/view/timeline/TKT-001"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestURLKanban(t *testing.T) {
	r := newURLWriter(t)
	got := evalString(t, r, `rela.url.kanban("sprint")`)
	if got != "/kanban/sprint" {
		t.Errorf("got %q, want /kanban/sprint", got)
	}
}

func TestURLDocument(t *testing.T) {
	r := newURLWriter(t)
	got := evalString(t, r, `rela.url.document("release_notes", {id="REL-001", type="release"})`)
	want := "/document/release_notes/REL-001"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// Singleton-route helpers — one per route in the frontend catalog that
// has no params. Each takes an optional bare query table.
func TestURLSingletons(t *testing.T) {
	r := newURLWriter(t)
	cases := []struct{ code, want string }{
		{`rela.url.home()`, "/dashboard"},
		{`rela.url.search()`, "/search"},
		{`rela.url.search({q = "pseudoniem"})`, "/search?q=pseudoniem"},
		{`rela.url.analyze()`, "/analyze"},
		{`rela.url.settings()`, "/settings"},
		{`rela.url.conflicts()`, "/conflicts"},
	}
	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			got := evalString(t, r, tc.code)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// Param-validation tests routed through form_create, which uses the
// same merge path as every non-form helper's bare query. Covers the
// edge cases the old call-style tests exercised.
func TestURLParams_validation(t *testing.T) {
	r := newURLWriter(t)
	cases := []struct {
		name    string
		code    string
		wantSub string
	}{
		{
			name:    "non-table opts arg",
			code:    `rela.url.form_create("full_ticket", "not a table")`,
			wantSub: "table",
		},
		{
			name:    "reserved key return_to",
			code:    `rela.url.list("all_tasks", {return_to="/evil"})`,
			wantSub: "reserved",
		},
		{
			name:    "key with &",
			code:    `rela.url.list("all_tasks", {["a&b"]="1"})`,
			wantSub: "forbidden characters",
		},
		{
			name:    "key with =",
			code:    `rela.url.list("all_tasks", {["a=b"]="1"})`,
			wantSub: "forbidden characters",
		},
		{
			name:    "empty key",
			code:    `rela.url.list("all_tasks", {[""]="1"})`,
			wantSub: "empty",
		},
		{
			name:    "function value",
			code:    `rela.url.list("all_tasks", {a=function() end})`,
			wantSub: "value must be string, number, or boolean",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := r.RunString(tc.code)
			if err == nil {
				t.Fatalf("expected error containing %q", tc.wantSub)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("error %v does not contain %q", err, tc.wantSub)
			}
		})
	}
}

// The form_create "with relations + properties + query" case used the
// old opts.query wrapper. The bare-query shape is for non-form helpers
// only; form_create keeps the three-sub-key opts shape because it has
// genuinely three semantics (rel./prop./passthrough).
func TestURLFormCreate_queryIsStillSubKey(t *testing.T) {
	r := newURLWriter(t)
	got := evalString(t, r,
		`rela.url.form_create("full_ticket", {
			relations = {parent = "TKT-1"},
			properties = {status = "open"},
			query = {source = "doc"},
		})`)
	want := "/form/full_ticket?prop.status=open&rel.parent=TKT-1&source=doc"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// Bare-query helpers produce deterministic sorted output and handle
// edge cases (empty table, numeric/bool values, unicode, existing
// query on path — not applicable here since helper builds its own).
func TestURLNonFormHelpers_querySemantics(t *testing.T) {
	r := newURLWriter(t)
	cases := []struct{ code, want string }{
		{`rela.url.list("all_tasks", {})`, "/list/all_tasks"},
		{`rela.url.list("all_tasks", {b="2", a="1", c="3"})`, "/list/all_tasks?a=1&b=2&c=3"},
		{`rela.url.list("all_tasks", {page=3, draft=true})`, "/list/all_tasks?draft=true&page=3"},
	}
	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			got := evalString(t, r, tc.code)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
