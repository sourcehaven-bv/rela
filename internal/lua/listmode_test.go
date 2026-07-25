package lua

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// sliceRows is a test ListRows over a fixed slice. Mirrors the adapter the
// data-entry export handler supplies in production.
type sliceRows []*entity.Entity

func (s sliceRows) Len() int { return len(s) }

func (s sliceRows) At(i int) *entity.Entity {
	if i < 0 || i >= len(s) {
		return nil
	}
	return s[i]
}

func testRows() sliceRows {
	return sliceRows{
		{ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "First"}},
		{ID: "TKT-2", Type: "ticket", Properties: map[string]any{"title": "Second"}},
		{ID: "TKT-3", Type: "ticket", Properties: map[string]any{"title": "Third"}},
	}
}

func testListContext() ListRenderContext {
	return ListRenderContext{
		ListID: "tickets",
		Rows:   testRows(),
		Query: ListQuery{
			EntityType: "ticket",
			Q:          "urgent",
			Filters:    map[string]string{"status": "open"},
			Sort:       []ListSortSpec{{Property: "title", Direction: "asc"}},
			Total:      3,
		},
	}
}

// runListDoc runs src in a list-document runtime and returns captured stdout.
func runListDoc(t *testing.T, lrc ListRenderContext, src string) string {
	t.Helper()
	ws := newMockWorkspace(t)
	var buf bytes.Buffer
	r := NewReader(ws.readDeps(t.TempDir()), &buf,
		WithListDocumentMode("export:list:tickets", lrc))
	defer r.Close()

	if err := r.RunString(src); err != nil {
		t.Fatalf("run: %v\ncaptured: %s", err, buf.String())
	}
	return buf.String()
}

// TestListDocumentMode_Surface pins the rela.document fields a list render
// sees, including the two that differ from an entity render.
func TestListDocumentMode_Surface(t *testing.T) {
	t.Parallel()

	got := runListDoc(t, testListContext(), `
print("mode=" .. rela.mode)
print("list_id=" .. rela.document.list_id)
print("entity_type=" .. rela.document.entity_type)
print("id=" .. rela.document.id)
print("count=" .. rela.document.count)
print("total=" .. rela.document.total)
print("truncated=" .. tostring(rela.document.truncated))
print("q=" .. rela.document.query.q)
print("status=" .. rela.document.query.filters.status)
print("sort=" .. rela.document.query.sort[1].property .. ":" .. rela.document.query.sort[1].direction)
`)

	for _, want := range []string{
		// mode stays "document": a list render reuses document mode
		// wholesale rather than introducing a third mode.
		"mode=document",
		"list_id=tickets",
		"entity_type=ticket",
		"id=export:list:tickets",
		"count=3",
		"total=3",
		"truncated=false",
		"q=urgent",
		"status=open",
		"sort=title:asc",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in output:\n%s", want, got)
		}
	}
}

// TestListDocumentMode_EntryIDAbsent is the regression test for the
// empty-string footgun: entry_id must be Lua nil in list mode so the
// idiomatic truthiness guard takes the correct branch and `or default`
// yields the default.
func TestListDocumentMode_EntryIDAbsent(t *testing.T) {
	t.Parallel()

	got := runListDoc(t, testListContext(), `
print("type=" .. type(rela.document.entry_id))
print("guard=" .. tostring(rela.document.entry_id ~= nil))
print("default=" .. (rela.document.entry_id or "fallback"))
`)

	for _, want := range []string{"type=nil", "guard=false", "default=fallback"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in output:\n%s", want, got)
		}
	}
}

// TestDocumentMode_EntryIDStillPresent guards the other direction: making
// entry_id conditional must not change an ENTITY render.
func TestDocumentMode_EntryIDStillPresent(t *testing.T) {
	t.Parallel()

	ws := newMockWorkspace(t)
	var buf bytes.Buffer
	r := NewReader(ws.readDeps(t.TempDir()), &buf, WithDocumentMode("book_card", "TKT-9"))
	defer r.Close()

	if err := r.RunString(`print("entry=" .. rela.document.entry_id)`); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(buf.String(), "entry=TKT-9") {
		t.Errorf("entity-mode entry_id changed: %s", buf.String())
	}
	// And a list render's row bindings must NOT appear on an entity render.
	buf.Reset()
	if err := r.RunString(`print("rows=" .. type(rela.document.rows))`); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(buf.String(), "rows=nil") {
		t.Errorf("entity render exposed list row bindings: %s", buf.String())
	}
}

// TestListDocumentMode_RowAccess covers row(i) bounds and 1-based indexing.
func TestListDocumentMode_RowAccess(t *testing.T) {
	t.Parallel()

	got := runListDoc(t, testListContext(), `
print("first=" .. rela.document.row(1).id)
print("title=" .. rela.document.row(1).properties.title)
print("last=" .. rela.document.row(3).id)
print("over=" .. type(rela.document.row(4)))
print("zero=" .. type(rela.document.row(0)))
`)

	for _, want := range []string{
		"first=TKT-1", "title=First", "last=TKT-3", "over=nil", "zero=nil",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in output:\n%s", want, got)
		}
	}
}

// TestListDocumentMode_IteratorWalkableTwice pins the contract that each
// rows() call mints a fresh cursor. A script that walks once to compute a
// total and again to emit must see the full set both times.
func TestListDocumentMode_IteratorWalkableTwice(t *testing.T) {
	t.Parallel()

	got := runListDoc(t, testListContext(), `
local function walk()
  local ids = {}
  for _, row in rela.document.rows() do
    ids[#ids+1] = row.id
  end
  return table.concat(ids, ",")
end
print("first=" .. walk())
print("second=" .. walk())
`)

	if !strings.Contains(got, "first=TKT-1,TKT-2,TKT-3") {
		t.Errorf("first walk wrong:\n%s", got)
	}
	if !strings.Contains(got, "second=TKT-1,TKT-2,TKT-3") {
		t.Errorf("second walk did not restart — rows() must mint a fresh cursor:\n%s", got)
	}
}

// TestListDocumentMode_QueryFrozen asserts the read-only context is enforced
// rather than conventional.
func TestListDocumentMode_QueryFrozen(t *testing.T) {
	t.Parallel()

	got := runListDoc(t, testListContext(), `
local ok, err = pcall(function() rela.document.query.q = "tampered" end)
print("ok=" .. tostring(ok))
print("q=" .. rela.document.query.q)
local ok2 = pcall(function() rela.document.query.filters.status = "closed" end)
print("filters_ok=" .. tostring(ok2))
`)

	for _, want := range []string{"ok=false", "q=urgent", "filters_ok=false"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in output:\n%s", want, got)
		}
	}
}

// TestListDocumentMode_Truncation asserts the cap bookkeeping reaches the
// script, so an override can render its own "showing N of M" notice.
func TestListDocumentMode_Truncation(t *testing.T) {
	t.Parallel()

	lrc := testListContext()
	lrc.Rows = testRows()[:2]
	lrc.Query.Total = 57

	got := runListDoc(t, lrc, `
print(("showing %d of %d, truncated=%s"):format(
  rela.document.count, rela.document.total, tostring(rela.document.truncated)))
`)

	if !strings.Contains(got, "showing 2 of 57, truncated=true") {
		t.Errorf("truncation bookkeeping wrong:\n%s", got)
	}
}
