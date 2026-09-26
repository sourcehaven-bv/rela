package worldreader_test

import (
	"context"
	"errors"
	"iter"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/worldreader"
)

// boundFixture stores the shape of the BUG-6XTX0G report: a policy type with
// faces `concept` and `adopted`, and a faceless task type beside it.
//
//	POL-1: adopted only
//	POL-2: concept only
//	POL-3: adopted and concept
//	TSK-1: default face (faceless type)
func boundFixture(t *testing.T) (*memstore.MemStore, store.WorldScope) {
	t.Helper()
	ctx := context.Background()
	st := memstore.New()
	adopted, concept := mustFace(t, "adopted"), mustFace(t, "concept")
	for _, row := range []struct {
		id, typ, title string
		face           entity.Face
	}{
		{"POL-1", "policy", "adopted one", adopted},
		{"POL-2", "policy", "concept two", concept},
		{"POL-3", "policy", "adopted three", adopted},
		{"POL-3", "policy", "concept three", concept},
		{"TSK-1", "task", "a task", ""},
	} {
		e := entity.New(row.id, row.typ)
		e.SetString("title", row.title)
		e.Face = row.face
		require.NoError(t, st.CreateEntity(ctx, e), "seed %s@%s", row.id, row.face)
	}
	scope := store.NewWorldScope(map[string]store.TypeResolution{
		"policy": {Chain: []entity.Face{adopted, concept}, Fallback: store.FallbackExclude},
	})
	return st, scope
}

func mustFace(t *testing.T, name string) entity.Face {
	t.Helper()
	f, err := entity.ParseFace(name)
	require.NoError(t, err)
	return f
}

func fixedSource(b worldreader.Binding) worldreader.Source {
	return func(context.Context) (worldreader.Binding, error) { return b, nil }
}

func titles(t *testing.T, seq iter.Seq2[*entity.Entity, error]) []string {
	t.Helper()
	var out []string
	for e, err := range seq {
		require.NoError(t, err)
		out = append(out, e.ID+"="+e.Title())
	}
	slices.Sort(out)
	return out
}

func TestBoundReader_GetEntity(t *testing.T) {
	st, scope := boundFixture(t)
	world := worldreader.Binding{Scope: scope}
	tests := []struct {
		name    string
		binding worldreader.Binding
		ref     string
		want    string // title; "" means not found
	}{
		{"bare id resolves to the world's first choice", world, "POL-3", "adopted three"},
		{"bare id falls through the chain", world, "POL-2", "concept two"},
		{"bare id of an adopted-only entity", world, "POL-1", "adopted one"},
		{"explicit face is served literally", world, "POL-3@concept", "concept three"},
		{"explicit face that does not exist", world, "POL-1@concept", ""},
		{"faceless type is unaffected", world, "TSK-1", "a task"},
		{"default world has no row for a faced entity", worldreader.Binding{}, "POL-1", ""},
		{"explicit face works in the default world", worldreader.Binding{}, "POL-1@adopted", "adopted one"},
		{"denied world finds nothing", worldreader.Binding{Scope: scope, Denied: true}, "TSK-1", ""},
		{"invalid address is a miss", world, "not an id", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r, err := worldreader.NewBoundReader(st, fixedSource(tc.binding))
			require.NoError(t, err)
			e, err := r.GetEntity(context.Background(), tc.ref)
			if tc.want == "" {
				require.Error(t, err)
				assert.Nil(t, e)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, e.Title())
		})
	}
}

func TestBoundReader_ListEntities(t *testing.T) {
	st, scope := boundFixture(t)
	world := worldreader.Binding{Scope: scope}
	concept := mustFace(t, "concept")
	tests := []struct {
		name    string
		binding worldreader.Binding
		q       store.EntityQuery
		want    []string
	}{
		{
			name: "typed list serves one prime per entity", binding: world,
			q:    store.EntityQuery{Type: "policy"},
			want: []string{"POL-1=adopted one", "POL-2=concept two", "POL-3=adopted three"},
		},
		{
			name: "faceless type is unaffected", binding: world,
			q:    store.EntityQuery{Type: "task"},
			want: []string{"TSK-1=a task"},
		},
		{
			name: "a face set narrows the candidates before the world ranks", binding: world,
			q:    store.EntityQuery{Type: "policy", FaceIn: []entity.Face{concept}},
			want: []string{"POL-2=concept two", "POL-3=concept three"},
		},
		{
			name: "a query naming AllStates is passed through", binding: world,
			q:    store.EntityQuery{IDs: []string{"POL-3"}, AllStates: true},
			want: []string{"POL-3=adopted three", "POL-3=concept three"},
		},
		{
			name: "default world lists no faced rows", binding: worldreader.Binding{},
			q: store.EntityQuery{Type: "policy"},
		},
		{
			name: "denied world lists nothing", binding: worldreader.Binding{Scope: scope, Denied: true},
			q: store.EntityQuery{Type: "task"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r, err := worldreader.NewBoundReader(st, fixedSource(tc.binding))
			require.NoError(t, err)
			assert.Equal(t, tc.want, titles(t, r.ListEntities(context.Background(), tc.q)))
		})
	}
}

// faceHidingReader stands in for the ACL-gated reader of a principal who may
// not read the adopted face: it drops those rows from every result.
type faceHidingReader struct {
	*memstore.MemStore
	hidden entity.Face
}

func (r faceHidingReader) ListEntities(ctx context.Context, q store.EntityQuery) iter.Seq2[*entity.Entity, error] {
	return func(yield func(*entity.Entity, error) bool) {
		for e, err := range r.MemStore.ListEntities(ctx, q) {
			if err == nil && e.Face == r.hidden {
				continue
			}
			if !yield(e, err) {
				return
			}
		}
	}
}

// The face gate filters BEFORE the world ranks, as on the data-entry GET: a
// reader who may not see the adopted face gets the concept face of POL-3
// rather than a not-found, and still nothing for adopted-only POL-1.
func TestBoundReader_GetEntity_RanksOnlyReadableFaces(t *testing.T) {
	st, scope := boundFixture(t)
	inner := faceHidingReader{MemStore: st, hidden: mustFace(t, "adopted")}
	r, err := worldreader.NewBoundReader(inner, fixedSource(worldreader.Binding{Scope: scope}))
	require.NoError(t, err)

	e, err := r.GetEntity(context.Background(), "POL-3")
	require.NoError(t, err)
	assert.Equal(t, "concept three", e.Title())

	_, err = r.GetEntity(context.Background(), "POL-1")
	assert.ErrorIs(t, err, store.ErrNotFound)
}

func TestBoundReader_SourceErrorIsReturned(t *testing.T) {
	st, _ := boundFixture(t)
	boom := errors.New("config unavailable")
	r, err := worldreader.NewBoundReader(st, func(context.Context) (worldreader.Binding, error) {
		return worldreader.Binding{}, boom
	})
	require.NoError(t, err)

	_, err = r.GetEntity(context.Background(), "TSK-1")
	require.ErrorIs(t, err, boom, "an infrastructure failure must not read as a miss")

	var listErr error
	for _, err := range r.ListEntities(context.Background(), store.EntityQuery{Type: "task"}) {
		listErr = err
	}
	assert.ErrorIs(t, listErr, boom)
}

func TestNewBoundReader_RejectsNil(t *testing.T) {
	st, _ := boundFixture(t)
	_, err := worldreader.NewBoundReader(nil, fixedSource(worldreader.Binding{}))
	require.Error(t, err)
	_, err = worldreader.NewBoundReader(st, nil)
	assert.Error(t, err)
}
