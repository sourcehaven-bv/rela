package datamigration

import (
	"context"
	"errors"
	"iter"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// readsFixture builds a data-only migration (metaV1 on both sides) whose
// steps are given as YAML, plus a script FS holding scripts by path.
func readsFixture(t *testing.T, steps string, scripts map[string]string) (*File, fstest.MapFS) {
	t.Helper()
	fsys := fstest.MapFS{}
	for p, src := range scripts {
		fsys[p] = &fstest.MapFile{Data: []byte(src)}
	}
	data := mustFileYAML(t, metaV1(), metaV1(), steps)
	return mustParse(t, testName("reads"), data), fsys
}

// runReads applies f to st and returns the result of the first step.
func runReads(t *testing.T, st store.Store, f *File, fsys fstest.MapFS) StepResult {
	t.Helper()
	r := newTestRunner(t, Deps{Store: st, Meta: metaV1(), ScriptFS: fsys})
	res, err := r.Run(t.Context(), []*File{f}, true)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return res.Files[0].Steps[0]
}

// runReadsErr applies f to st and returns the run error, failing if there is
// none.
func runReadsErr(t *testing.T, st store.Store, script string) error {
	t.Helper()
	f, fsys := readsFixture(t,
		"  - lua: {entity: task, script: migrations/r.lua}\n",
		map[string]string{"migrations/r.lua": script})
	r := newTestRunner(t, Deps{Store: st, Meta: metaV1(), ScriptFS: fsys})
	_, err := r.Run(t.Context(), []*File{f}, false)
	if err == nil {
		t.Fatal("Run succeeded, want an error")
	}
	return err
}

// classifyByRelation is the case the reads exist for (atlas: a task whose
// gaat_over points to an ncr becomes a corrective action): classify each task
// by an outgoing relation to an entity of a given type.
const classifyByRelation = `
function migrate(e)
  local kind = "general"
  for _, r in ipairs(rela.get_relations({ from = e.id, type = "assigned-to" })) do
    local target = rela.get_entity(r.to)
    if target ~= nil and target.type == "person" then kind = "assigned" end
  end
  if e.properties.kind == kind then return nil end
  return { properties = { kind = kind } }
end`

// seeOthers patches every task's title and records, in `seen`, the titles of
// the other tasks as get_entity returned them. Each task reads every other
// task, so whatever order the store yields them in, some reads happen after
// the read entity was patched in this step.
const seeOthers = `
function migrate(e)
  local seen = {}
  for _, id in ipairs({ "TSK-1", "TSK-2", "TSK-3" }) do
    if id ~= e.id then
      table.insert(seen, id .. "=" .. rela.get_entity(id).properties.title)
    end
  end
  return { properties = { title = e.properties.title .. "!", seen = table.concat(seen, ",") } }
end`

// backendCase opens one storage backend for the cross-backend tests.
type backendCase struct {
	name string
	open func(*testing.T) store.Store
}

// readBackends lists every backend a migration runs on. The postgres case
// skips without RELA_TEST_DATABASE_URL (luareads_backends_test.go).
func readBackends() []backendCase {
	return append([]backendCase{
		{"memstore", func(*testing.T) store.Store { return memstore.New() }},
		{"fsstore", openFSStore},
		{"sqlitestore", openSQLiteStore},
	}, pgBackend()...)
}

func TestLuaReads_ClassifyByRelationOnEveryBackend(t *testing.T) {
	for _, b := range readBackends() {
		t.Run(b.name, func(t *testing.T) {
			st := seedInto(t, b.open(t))
			f, fsys := readsFixture(t,
				"  - lua: {entity: task, script: migrations/classify.lua}\n",
				map[string]string{"migrations/classify.lua": classifyByRelation})

			if got := runReads(t, st, f, fsys).Affected; got != 3 {
				t.Fatalf("affected = %d, want 3", got)
			}
			for id, want := range map[string]string{"TSK-1": "assigned", "TSK-2": "general", "TSK-3": "general"} {
				if got := getEntity(t, st, id).Properties["kind"]; got != want {
					t.Errorf("%s kind = %v, want %s", id, got, want)
				}
			}
			// Idempotent: the second run finds every task classified.
			if got := runReads(t, st, f, fsys).Affected; got != 0 {
				t.Errorf("re-run affected = %d, want 0", got)
			}
		})
	}
}

func TestLuaReads_SeePreStepStateInEitherOrder(t *testing.T) {
	cases := readBackends()
	// The same store with its listing reversed, so the step meets the tasks
	// in the opposite order.
	cases = append(cases, backendCase{"memstore-reversed", func(*testing.T) store.Store {
		return reversedLister{memstore.New()}
	}})
	for _, b := range cases {
		t.Run(b.name, func(t *testing.T) {
			st := seedInto(t, b.open(t))
			f, fsys := readsFixture(t,
				"  - lua: {entity: task, script: migrations/see.lua}\n",
				map[string]string{"migrations/see.lua": seeOthers})
			runReads(t, st, f, fsys)

			want := map[string]string{
				"TSK-1": "TSK-2=two,TSK-3=three",
				"TSK-2": "TSK-1=one,TSK-3=three",
				"TSK-3": "TSK-1=one,TSK-2=two",
			}
			for id, w := range want {
				e := getEntity(t, st, id)
				if got := e.Properties["seen"]; got != w {
					t.Errorf("%s saw %v, want %s (a read saw this step's patch)", id, got, w)
				}
				if !strings.HasSuffix(e.Properties["title"].(string), "!") {
					t.Errorf("%s title = %v, patch not applied", id, e.Properties["title"])
				}
			}
		})
	}
}

func TestLuaReads_LaterStepSeesEarlierStep(t *testing.T) {
	st := seedStore(t)
	f, fsys := readsFixture(t,
		"  - lua: {entity: person, script: migrations/upper.lua}\n"+
			"  - lua: {entity: task, script: migrations/copy.lua}\n",
		map[string]string{
			"migrations/upper.lua": `
function migrate(e)
  return { properties = { shout = string.upper(e.properties.name) } }
end`,
			"migrations/copy.lua": `
function migrate(e)
  for _, r in ipairs(rela.get_relations({ from = e.id, type = "assigned-to" })) do
    return { properties = { assignee = rela.get_entity(r.to).properties.shout } }
  end
  return nil
end`,
		})
	runReads(t, st, f, fsys)
	if got := getEntity(t, st, "TSK-1").Properties["assignee"]; got != "ADA" {
		t.Errorf("assignee = %v, want ADA (step 2 must read step 1's write)", got)
	}
}

func TestLuaReads_IgnoreSchemaAndValidation(t *testing.T) {
	st := seedStore(t)
	ctx := t.Context()
	// A type the schema does not know, and a task missing its required title.
	for _, e := range []*entity.Entity{
		{ID: "OLD-1", Type: "legacy", Properties: map[string]any{"x": "kept"}},
		{ID: "TSK-9", Type: "task", Properties: map[string]any{"status": "open"}},
	} {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	f, fsys := readsFixture(t,
		"  - lua: {entity: task, script: migrations/r.lua}\n",
		map[string]string{"migrations/r.lua": `
function migrate(e)
  if e.id ~= "TSK-1" then return nil end
  local old, bad = rela.get_entity("OLD-1"), rela.get_entity("TSK-9")
  return { properties = { got = old.type .. ":" .. old.properties.x .. "," .. bad.properties.status } }
end`})
	runReads(t, st, f, fsys)
	if got := getEntity(t, st, "TSK-1").Properties["got"]; got != "legacy:kept,open" {
		t.Errorf("got = %v, want legacy:kept,open", got)
	}
}

func TestLuaReads_WildcardStep(t *testing.T) {
	st := seedStore(t)
	f, fsys := readsFixture(t,
		"  - lua: {entity: '*', script: migrations/deg.lua}\n",
		map[string]string{"migrations/deg.lua": `
function migrate(e)
  return { properties = {
    out_degree = #rela.get_relations({ from = e.id }),
    in_degree = #rela.get_relations({ to = e.id }),
  } }
end`})
	if got := runReads(t, st, f, fsys).Affected; got != 4 {
		t.Fatalf("affected = %d, want 4", got)
	}
	for id, want := range map[string][2]int{"TSK-1": {1, 0}, "TSK-2": {0, 0}, "PER-1": {0, 1}} {
		p := getEntity(t, st, id).Properties
		if p["out_degree"] != want[0] || p["in_degree"] != want[1] {
			t.Errorf("%s out/in = %v/%v, want %d/%d", id, p["out_degree"], p["in_degree"], want[0], want[1])
		}
	}
}

func TestLuaReads_RelationTableShape(t *testing.T) {
	st := seedStore(t)
	f, fsys := readsFixture(t,
		"  - lua: {entity: task, script: migrations/r.lua}\n",
		map[string]string{"migrations/r.lua": `
function migrate(e)
  if e.id ~= "TSK-1" then return nil end
  local r = rela.get_relations({ from = "TSK-1", type = "assigned-to", to = "PER-1" })[1]
  local all = rela.get_relations()
  return { properties = { rel = r.from .. ">" .. r.type .. ">" .. r.to .. "|" .. r.from_face ..
    "|" .. r.properties.weight .. "|" .. r.content .. "|" .. #all } }
end`})
	runReads(t, st, f, fsys)
	if got := getEntity(t, st, "TSK-1").Properties["rel"]; got != "TSK-1>assigned-to>PER-1||high||1" {
		t.Errorf("rel = %v", got)
	}
}

func TestLuaReads_OnlyReadsAreExposed(t *testing.T) {
	st := seedStore(t)
	f, fsys := readsFixture(t,
		"  - lua: {entity: task, script: migrations/r.lua}\n",
		map[string]string{"migrations/r.lua": `
local names = {}
for k in pairs(rela) do table.insert(names, k) end
table.sort(names)
local exposed = table.concat(names, ",")
function migrate(e)
  return { properties = { exposed = exposed } }
end`})
	runReads(t, st, f, fsys)
	if got := getEntity(t, st, "TSK-1").Properties["exposed"]; got != "get_entity,get_faces,get_relations" {
		t.Errorf("rela exposes %v, want only get_entity,get_faces,get_relations", got)
	}
}

func TestLuaReads_Faces(t *testing.T) {
	st := seedStore(t)
	ctx := t.Context()
	for _, e := range []*entity.Entity{
		{ID: "DOC-1", Type: "doc", Face: entity.Face("draft"), Properties: map[string]any{"v": "d"}},
		{ID: "DOC-1", Type: "doc", Face: entity.Face("published"), Properties: map[string]any{"v": "p"}},
		{ID: "DOC-2", Type: "doc", Face: entity.Face("draft"), Properties: map[string]any{"v": "only"}},
	} {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	f, fsys := readsFixture(t,
		"  - lua: {entity: task, script: migrations/r.lua}\n",
		map[string]string{"migrations/r.lua": `
function migrate(e)
  if e.id ~= "TSK-1" then return nil end
  local named = rela.get_entity("DOC-1@published")
  local single = rela.get_entity("DOC-2")
  local missing = rela.get_entity("NOPE-1")
  local missingFace = rela.get_entity("DOC-2@published")
  return { properties = { faces = named.face .. "=" .. named.properties.v .. "," ..
    single.face .. "=" .. single.properties.v .. "," ..
    tostring(missing) .. "," .. tostring(missingFace) } }
end`})
	runReads(t, st, f, fsys)
	if got := getEntity(t, st, "TSK-1").Properties["faces"]; got != "published=p,draft=only,nil,nil" {
		t.Errorf("faces = %v", got)
	}

	err := runReadsErr(t, st, `
function migrate(e) return { properties = { x = rela.get_entity("DOC-1").id } } end`)
	if !strings.Contains(err.Error(), "draft, published") || !strings.Contains(err.Error(), "DOC-1@<face>") {
		t.Errorf("ambiguous bare id error = %v, want it to name the faces", err)
	}
}

func TestLuaReads_RejectBadArguments(t *testing.T) {
	for name, tc := range map[string]struct{ call, want string }{
		"non-string option": {`rela.get_relations({ from = 12 })`, `option "from" must be a string`},
		"bare id as filter": {`rela.get_relations("TSK-1")`, "must be an options table"},
		"empty id":          {`rela.get_entity("")`, "cannot be empty"},
		"number id":         {`rela.get_entity(12)`, "id must be a string, got number"},
		"malformed address": {`rela.get_entity("TSK-1@")`, "invalid"},
		"unknown option":    {`rela.get_relations({ form = "TSK-1" })`, `unknown option form`},
		"nil filter value":  {`rela.get_relations({ from = e.properties.nope })`, "empty options table"},
		"empty string":      {`rela.get_relations({ from = "" })`, `option "from" is empty`},
		"faces of address":  {`rela.get_faces("TSK-1@nl")`, "takes a bare id"},
	} {
		t.Run(name, func(t *testing.T) {
			err := runReadsErr(t, seedStore(t), "function migrate(e) local _ = "+tc.call+" return nil end")
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

// reversedLister is a store whose entity listing runs backwards.
type reversedLister struct{ store.Store }

func (r reversedLister) ListEntities(ctx context.Context, q store.EntityQuery) iter.Seq2[*entity.Entity, error] {
	var rows []*entity.Entity
	for e, err := range r.Store.ListEntities(ctx, q) {
		if err != nil {
			return func(yield func(*entity.Entity, error) bool) { yield(nil, err) }
		}
		rows = append(rows, e)
	}
	slices.Reverse(rows)
	return func(yield func(*entity.Entity, error) bool) {
		for _, e := range rows {
			if !yield(e, nil) {
				return
			}
		}
	}
}

func TestLuaReads_OwnRowIsPreStep(t *testing.T) {
	st := seedStore(t)
	f, fsys := readsFixture(t,
		"  - lua: {entity: task, script: migrations/r.lua}\n",
		map[string]string{"migrations/r.lua": `
function migrate(e)
  e.properties.title = "mutated in place"
  return { properties = { stored = rela.get_entity(e.id).properties.title } }
end`})
	runReads(t, st, f, fsys)
	if got := getEntity(t, st, "TSK-1").Properties["stored"]; got != "one" {
		t.Errorf("stored = %v, want one (the read must not see the handed table)", got)
	}
}

func TestLuaReads_DryRunReadsThePreRunStore(t *testing.T) {
	st := seedStore(t)
	f, fsys := readsFixture(t,
		"  - lua: {entity: person, script: migrations/a.lua}\n"+
			"  - lua: {entity: task, script: migrations/b.lua}\n",
		map[string]string{
			"migrations/a.lua": `function migrate(e) return { properties = { shout = "ADA" } } end`,
			// Changes a task only if step 1's write is visible.
			"migrations/b.lua": `
function migrate(e)
  if rela.get_entity("PER-1").properties.shout == nil then return nil end
  return { properties = { seen = true } }
end`,
		})
	r := newTestRunner(t, Deps{Store: st, Meta: metaV1(), ScriptFS: fsys})
	res, err := r.Run(t.Context(), []*File{f}, false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := res.Files[0].Steps[1].Affected; got != 0 {
		t.Errorf("dry-run step 2 affected = %d, want 0: a dry run writes nothing, so step 1 is invisible", got)
	}
}

func TestLuaReads_StepFaceAndFaces(t *testing.T) {
	st := memstore.New()
	ctx := t.Context()
	for _, e := range []*entity.Entity{
		{ID: "TSK-1", Type: "task", Face: entity.Face("en"), Properties: map[string]any{"title": "one"}},
		{ID: "TSK-1", Type: "task", Face: entity.Face("nl"), Properties: map[string]any{"title": "een"}},
		{ID: "PER-1", Type: "person", Properties: map[string]any{"name": "Ada"}},
	} {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	// Only the nl row is assigned.
	if _, err := st.CreateRelation(ctx,
		entity.RelationKey{From: "TSK-1", FromFace: entity.Face("nl"), Type: "assigned-to", To: "PER-1"},
		&store.RelationData{}); err != nil {
		t.Fatal(err)
	}
	f, fsys := readsFixture(t,
		"  - lua: {entity: task, script: migrations/r.lua}\n",
		map[string]string{"migrations/r.lua": `
function migrate(e)
  local mine = 0
  for _, r in ipairs(rela.get_relations({ from = e.id })) do
    if r.from_face == e.face then mine = mine + 1 end
  end
  local faces = {}
  for _, row in ipairs(rela.get_faces(e.id)) do
    table.insert(faces, row.face .. "=" .. row.properties.title)
  end
  return { properties = { mine = mine, faces = table.concat(faces, ",") } }
end`})
	runReads(t, st, f, fsys)
	for face, want := range map[string]int{"en": 0, "nl": 1} {
		e, err := st.GetEntity(ctx, entity.Ref{ID: "TSK-1", Face: entity.Face(face)})
		if err != nil {
			t.Fatal(err)
		}
		if e.Properties["mine"] != want {
			t.Errorf("TSK-1@%s mine = %v, want %d", face, e.Properties["mine"], want)
		}
		if e.Properties["faces"] != "en=one,nl=een" {
			t.Errorf("TSK-1@%s faces = %v", face, e.Properties["faces"])
		}
	}
}

// failingRelations is a reader whose relation listing fails.
type failingRelations struct{ store.Store }

func (failingRelations) ListRelations(context.Context, store.RelationQuery) iter.Seq2[*entity.Relation, error] {
	return func(yield func(*entity.Relation, error) bool) { yield(nil, errors.New("disk on fire")) }
}

func TestLuaReads_StoreErrorRaises(t *testing.T) {
	err := runReadsErr(t, failingRelations{seedStore(t)},
		`function migrate(e) local _ = rela.get_relations({ from = e.id }) return nil end`)
	if !strings.Contains(err.Error(), "disk on fire") {
		t.Errorf("error = %v, want the store error", err)
	}
}
