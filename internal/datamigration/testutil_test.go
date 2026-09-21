package datamigration

import (
	"context"
	"fmt"
	"io/fs"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// fakeKV is an in-memory state.KV for tests.
type fakeKV struct {
	mu   sync.Mutex
	data map[string][]byte
}

func newFakeKV() *fakeKV { return &fakeKV{data: map[string][]byte{}} }

func (f *fakeKV) Get(_ context.Context, key string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.data[key]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return v, nil
}

func (f *fakeKV) Put(_ context.Context, key string, data []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.data[key] = append([]byte(nil), data...)
	return nil
}

func (f *fakeKV) Delete(_ context.Context, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.data, key)
	return nil
}

// metaV1 is the baseline test schema: task {title, status(enum via named
// type), due(date), tags(list)}, person {name}, relation assigned-to.
func metaV1() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Types: map[string]metamodel.CustomType{
			"status": {Values: []string{"open", "wip", "done"}},
		},
		Entities: map[string]metamodel.EntityDef{
			"task": {
				Properties: map[string]metamodel.PropertyDef{
					"title":  {Type: "string", Required: true},
					"status": {Type: "status"},
					"due":    {Type: "string"},
					"tags":   {Type: "string", List: true},
				},
			},
			"person": {
				Properties: map[string]metamodel.PropertyDef{
					"name": {Type: "string", Required: true},
				},
			},
		},
		Relations: map[string]metamodel.RelationDef{
			"assigned-to": {
				From:       []string{"task"},
				To:         []string{"person"},
				Properties: map[string]metamodel.PropertyDef{"weight": {Type: "string"}},
			},
		},
	}
}

// metaV2 is metaV1 after an incompatible evolution: status→state renamed
// (delete+add of same shape) with the enum values remapped, due converted to
// a date type.
func metaV2() *metamodel.Metamodel {
	m := metaV1()
	m.Types["status"] = metamodel.CustomType{Values: []string{"todo", "doing", "done"}}
	props := m.Entities["task"].Properties
	delete(props, "status")
	props["state"] = metamodel.PropertyDef{Type: "status"}
	props["due"] = metamodel.PropertyDef{Type: "date"}
	return m
}

// mustFileYAML renders a well-formed migration file from two metamodels and a
// steps block, with embedded projections.
//
// No from/to hashes: a migration is identified by NAME now, and the
// projections are what step validation and validateDeltasResolved run on.
func mustFileYAML(t *testing.T, from, to *metamodel.Metamodel, stepsYAML string) []byte {
	t.Helper()
	fp := from.ShapeProjection()
	tp := to.ShapeProjection()
	fromYAML, err := marshalProjectionYAML("from_projection", fp)
	if err != nil {
		t.Fatalf("marshal from projection: %v", err)
	}
	toYAML, err := marshalProjectionYAML("to_projection", tp)
	if err != nil {
		t.Fatalf("marshal to projection: %v", err)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "description: test\nsteps:\n%s%s%s", stepsYAML, fromYAML, toYAML)
	return []byte(b.String())
}

// testName builds a valid migration filename for a test.
//
// Every name must satisfy the MigrationName allowlist now, so tests cannot
// use ad-hoc strings like "0001-x.yaml".
func testName(slug string) string {
	return "20260919143022-" + slug + ".yaml"
}

// fakeMigState is an in-memory StateStore for tests.
//
// Hand-rolled rather than reusing memmigstate: that package imports
// datamigration, and these tests are IN package datamigration, so importing it
// would be an import cycle. memmigstate is held to the same contract by
// migstatetest.
type fakeMigState struct {
	mu    sync.Mutex
	state *State
}

func (f *fakeMigState) Load(context.Context) (*State, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.state == nil {
		return nil, nil //nolint:nilnil // (nil, nil) = un-bootstrapped, the StateStore contract
	}
	out := *f.state
	return &out, nil
}

func (f *fakeMigState) Save(_ context.Context, s *State) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := *s
	f.state = &out
	return nil
}

// newMigState returns an empty in-memory migration-state store.
func newMigState() *fakeMigState { return &fakeMigState{} }

func mustParse(t *testing.T, name string, data []byte) *File {
	t.Helper()
	f, err := ParseFile(name, data)
	if err != nil {
		t.Fatalf("ParseFile(%s): %v", name, err)
	}
	return f
}

// seedStore builds a memstore with representative v1 data.
func seedStore(t *testing.T) store.Store {
	t.Helper()
	st := memstore.New()
	ctx := t.Context()
	entities := []*entity.Entity{
		{ID: "TSK-1", Type: "task", Properties: map[string]any{"title": "one", "status": "open", "due": "01/02/2026"}},
		{ID: "TSK-2", Type: "task", Properties: map[string]any{"title": "two", "status": "wip"}},
		{ID: "TSK-3", Type: "task", Properties: map[string]any{"title": "three", "status": "done", "due": "2026-03-04"}},
		{ID: "PER-1", Type: "person", Properties: map[string]any{"name": "Ada"}},
	}
	for _, e := range entities {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatalf("seed %s: %v", e.ID, err)
		}
	}
	if _, err := st.CreateRelation(ctx, "TSK-1", "assigned-to", "PER-1",
		&store.RelationData{Properties: map[string]any{"weight": "high"}}); err != nil {
		t.Fatalf("seed relation: %v", err)
	}
	return st
}

func getEntity(t *testing.T, st store.Store, id string) *entity.Entity {
	t.Helper()
	e, err := st.GetEntity(t.Context(), id)
	if err != nil {
		t.Fatalf("GetEntity(%s): %v", id, err)
	}
	return e
}

// emptyFS is a ScriptFS for runs without lua steps.
var emptyFS = fstest.MapFS{}
