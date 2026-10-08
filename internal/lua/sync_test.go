package lua_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

const syncRefSchema = `version: "1.0"
entities:
  ticket:
    label: Ticket
    id_prefix: "T-"
    properties:
      title: {type: string}
      basecamp: {type: external_ref, system: basecamp}
      jira: {type: external_ref, system: jira, sync: true}
`

func syncRuntime(t *testing.T, seed ...*entity.Entity) (*lua.Runtime, *bytes.Buffer) {
	t.Helper()
	meta, err := metamodel.Parse([]byte(syncRefSchema))
	if err != nil {
		t.Fatal(err)
	}
	st := memstore.New()
	for _, e := range seed {
		if err := st.CreateEntity(context.Background(), e); err != nil {
			t.Fatal(err)
		}
	}
	var out bytes.Buffer
	rt := lua.NewReader(lua.ReadDeps{
		Meta:          meta,
		World:         store.TrivialScope(),
		VisibleReader: visibility.Unrestricted(st).WithWorld(visibility.WorldOf(store.TrivialScope())),
	}, &out)
	t.Cleanup(rt.Close)
	return rt, &out
}

func refEntity(id, prop, ext string) *entity.Entity {
	e := entity.New(id, "ticket")
	e.Properties[prop] = map[string]any{"id": ext}
	return e
}

// AC7: ambiguity raises, counted after visibility; a sync system on a
// runtime without history raises (D11).
func TestFindByExternalRef_Raises(t *testing.T) {
	rt, _ := syncRuntime(t, refEntity("T-1", "basecamp", "1"), refEntity("T-2", "basecamp", "1"))
	err := rt.RunString(`rela.find_by_external_ref("basecamp", "1")`)
	if err == nil || !strings.Contains(err.Error(), "2 entities") {
		t.Fatalf("err = %v, want an ambiguity error", err)
	}
	err = rt.RunString(`rela.find_by_external_ref("jira", "1")`)
	if err == nil || !strings.Contains(err.Error(), "sync needs version history") {
		t.Fatalf("err = %v, want the history refusal", err)
	}
	err = rt.RunString(`rela.sync.merge(nil, {type = "ticket", properties = {}}, {}, {fields = {"title"}})`)
	if err == nil || !strings.Contains(err.Error(), "sync needs version history") {
		t.Fatalf("err = %v, want the history refusal", err)
	}
}

func TestSyncEmptyIsUnforgeable(t *testing.T) {
	rt, out := syncRuntime(t)
	if err := rt.RunString(`
rela.output(tostring(rela.sync.EMPTY == rela.sync.EMPTY))
rela.output(tostring(rela.sync.EMPTY == {}))
rela.output(type(rela.sync.EMPTY))
`); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); !strings.Contains(got, "true") || !strings.Contains(got, "false") || !strings.Contains(got, "userdata") {
		t.Fatalf("output = %q", got)
	}
}

// emptyHistory makes a reader sync-ready without serving any version.
type emptyHistory struct{}

func (emptyHistory) ListVersions(context.Context, entity.Ref) ([]store.VersionMeta, error) {
	return nil, nil
}

func (emptyHistory) GetVersion(context.Context, entity.Ref, int) (*store.VersionSnapshot, error) {
	return nil, store.ErrNotFound
}

func (emptyHistory) VersionByTag(context.Context, entity.Ref, store.VersionTagName) (*store.VersionSnapshot, error) {
	return nil, store.ErrNotFound
}

// AC8/AC9 through the binding: the result table's shape, EMPTY in write and
// push, canonical values, and base nil reporting every difference as a
// conflict.
func TestSyncMerge_ResultTable(t *testing.T) {
	meta, err := metamodel.Parse([]byte(syncRefSchema + `      points: {type: integer}
      due: {type: date}
`))
	if err != nil {
		t.Fatal(err)
	}
	rd := visibility.Unrestricted(memstore.New()).WithWorld(visibility.WorldOf(store.TrivialScope())).
		WithHistory(emptyHistory{}).WithVersionTags(emptyHistory{})
	var out bytes.Buffer
	rt := lua.NewReader(lua.ReadDeps{Meta: meta, World: store.TrivialScope(), VisibleReader: rd}, &out)
	t.Cleanup(rt.Close)

	err = rt.RunString(`
local base = {type = "ticket", properties = {title = "a", points = 1, due = "2026-01-01"}, content = "x"}
local ours = {type = "ticket", properties = {title = "a", points = 2, due = "2026-01-01"}, content = "x\n"}
local r = rela.sync.merge(base, ours,
  {properties = {title = "b", points = "1", due = rela.sync.EMPTY}, content = "x"},
  {fields = {"title", "points", "due", "content"}})
rela.output("write.title=" .. tostring(r.write.title))
rela.output("write.due_empty=" .. tostring(r.write.due == rela.sync.EMPTY))
rela.output("push.points=" .. tostring(r.push.points))
rela.output("unchanged=" .. table.concat(r.unchanged, ","))
rela.output("retag=" .. tostring(r.retag))
rela.output("complete=" .. tostring(r.complete))
local p = rela.sync.merge(base, ours, {properties = {title = "a"}}, {fields = {"title", "points"}})
rela.output("partial_complete=" .. tostring(p.complete))
local n = rela.sync.merge(nil, ours, {properties = {points = 5}}, {fields = {"points"}})
rela.output("base_unknown=" .. tostring(n.base_unknown))
rela.output("conflict=" .. n.conflicts[1].field .. ":" .. tostring(n.conflicts[1].ours) .. ":" .. tostring(n.conflicts[1].theirs))
local ok = pcall(rela.sync.merge, nil, ours, {properties = {}}, {})
rela.output("no_fields=" .. tostring(ok))
ok = pcall(rela.sync.merge, "x", ours, {properties = {}}, {fields = {"title"}})
rela.output("bad_base=" .. tostring(ok))
ok = pcall(rela.sync.merge, base, ours, {properties = {}, content = 5}, {fields = {"content"}})
rela.output("numeric_content=" .. tostring(ok))
ok = pcall(rela.sync.merge, base, ours, {properties = {}, content = {}}, {fields = {"content"}})
rela.output("table_content=" .. tostring(ok))
local e = rela.sync.merge(base, ours, {properties = {}, content = rela.sync.EMPTY}, {fields = {"content"}})
rela.output("empty_content_write=[" .. tostring(e.content) .. "]")
`)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"write.title=b", "write.due_empty=true", "push.points=2", "unchanged=content", "retag=false",
		"complete=true", "partial_complete=false", "numeric_content=false", "table_content=false",
		"empty_content_write=[]",
		"base_unknown=true", "conflict=points:2:5", "no_fields=false", "bad_base=false",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, out.String())
		}
	}
}
