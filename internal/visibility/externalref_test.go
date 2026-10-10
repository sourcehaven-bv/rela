package visibility_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
	"github.com/Sourcehaven-BV/rela/internal/visibility/visibilitytest"
)

// setGate admits exactly the ids it holds.
type setGate map[string]bool

func (g setGate) PermitsRead(_ context.Context, _, id string) (bool, error) { return g[id], nil }

func (g setGate) ReadableFacesMany(_ context.Context, _ string, ids []string) (acl.FaceVerdicts, error) {
	out := make(map[string]bool, len(ids))
	for _, id := range ids {
		out[id] = g[id]
	}
	return visibilitytest.IDVerdicts(out, nil)
}

// refRedactor hides the external ref of one entity.
type refRedactor struct{ id, prop string }

func (r refRedactor) HiddenProperties(_ context.Context, e *entity.Entity) map[string]struct{} {
	if e.ID == r.id {
		return map[string]struct{}{r.prop: {}}
	}
	return nil
}

// noTags serves no tag; only its presence matters to SyncReady.
type noTags struct{}

func (noTags) VersionByTag(context.Context, entity.Ref, store.VersionTagName) (*store.VersionSnapshot, error) {
	return nil, store.ErrNotFound
}

var refHolders = []visibility.ExternalRefHolder{
	{Type: "ticket", Property: "bc"},
	{Type: "vault", Property: "v"},
}

func externalRefFixture(t *testing.T) (*visibility.ScriptReader, *visibility.UnrestrictedReader) {
	t.Helper()
	st := memstore.New()
	seed := func(id, typ, prop, ext string) {
		e := entity.New(id, typ)
		e.Properties[prop] = map[string]any{"id": ext}
		if err := st.CreateEntity(context.Background(), e); err != nil {
			t.Fatal(err)
		}
	}
	seed("TKT-1", "ticket", "bc", "1")
	seed("TKT-2", "ticket", "bc", "2") // row hidden
	seed("TKT-3", "ticket", "bc", "3") // ref field hidden
	seed("VLT-1", "vault", "v", "1")   // readable: makes "1" ambiguous
	seed("VLT-2", "vault", "v", "2")   // row hidden
	gate := setGate{"TKT-1": true, "TKT-3": true, "VLT-1": true}
	policy, err := visibility.NewPolicyReader(gate, refRedactor{id: "TKT-3", prop: "bc"}, st)
	if err != nil {
		t.Fatal(err)
	}
	script, err := visibility.NewScriptReader(policy, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	world := visibility.WorldOf(store.TrivialScope())
	return script.WithWorld(world), visibility.Unrestricted(st).WithWorld(world)
}

func foundIDs(t *testing.T, got []*entity.Entity, err error) []string {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, 0, len(got))
	for _, e := range got {
		ids = append(ids, e.ID)
	}
	return ids
}

// AC7 (TKT-SM20FG): the lookup answers only what the reader may read. A
// hidden row and a hidden ref field both read as no match, and ambiguity is
// counted after that filter.
func TestFindByExternalRef_GatedAndRedacted(t *testing.T) {
	script, unrestricted := externalRefFixture(t)
	ctx := context.Background()
	tests := []struct {
		name   string
		id     string
		script []string
		all    []string
	}{
		{name: "readable in two types", id: "1", script: []string{"TKT-1", "VLT-1"}, all: []string{"TKT-1", "VLT-1"}},
		{name: "hidden rows", id: "2", script: []string{}, all: []string{"TKT-2", "VLT-2"}},
		{name: "hidden ref field", id: "3", script: []string{}, all: []string{"TKT-3"}},
		{name: "unknown id", id: "9", script: []string{}, all: []string{}},
		{name: "empty id", id: "", script: []string{}, all: []string{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := script.FindByExternalRef(ctx, refHolders, tc.id)
			if ids := foundIDs(t, got, err); !slices.Equal(ids, tc.script) {
				t.Errorf("script reader = %v, want %v", ids, tc.script)
			}
			got, err = unrestricted.FindByExternalRef(ctx, refHolders, tc.id)
			if ids := foundIDs(t, got, err); !slices.Equal(ids, tc.all) {
				t.Errorf("unrestricted reader = %v, want %v", ids, tc.all)
			}
		})
	}
	got, err := visibility.DenyReader{}.FindByExternalRef(ctx, refHolders, "1")
	if len(got) != 0 || err != nil {
		t.Errorf("DenyReader = %v, %v; want nothing", got, err)
	}
}

// D11: a sync holder needs history and tags on the reader.
func TestFindByExternalRef_SyncNeedsHistory(t *testing.T) {
	script, unrestricted := externalRefFixture(t)
	ctx := context.Background()
	sync := []visibility.ExternalRefHolder{{Type: "ticket", Property: "bc", Sync: true}}

	if _, err := script.FindByExternalRef(ctx, sync, "1"); !errors.Is(err, store.ErrHistoryUnsupported) {
		t.Errorf("script reader without history: err = %v", err)
	}
	if _, err := unrestricted.FindByExternalRef(ctx, sync, "1"); !errors.Is(err, store.ErrHistoryUnsupported) {
		t.Errorf("unrestricted reader without history: err = %v", err)
	}
	if script.SyncReady() || unrestricted.SyncReady() {
		t.Error("SyncReady without history")
	}

	h := cannedHistory{}
	script = script.WithHistory(h).WithVersionTags(noTags{})
	unrestricted = unrestricted.WithHistory(h).WithVersionTags(noTags{})
	if !script.SyncReady() || !unrestricted.SyncReady() {
		t.Error("SyncReady with history and tags")
	}
	got, err := script.FindByExternalRef(ctx, sync, "1")
	if ids := foundIDs(t, got, err); !slices.Equal(ids, []string{"TKT-1"}) {
		t.Errorf("script reader with history = %v", ids)
	}
}

// Finding 7: on a faced type the candidate is read at the face that holds
// the ref, so a family with several faces is neither skipped nor ambiguous.
func TestFindByExternalRef_Faces(t *testing.T) {
	st := memstore.New()
	ctx := context.Background()
	seed := func(id string, face entity.Face, ext string) {
		t.Helper()
		e := &entity.Entity{ID: id, Type: "page", Face: face, Properties: map[string]any{}}
		if ext != "" {
			e.Properties["jira"] = map[string]any{"id": ext}
		}
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	seed("PAGE-1", "live", "J-1")
	seed("PAGE-1", "review", "J-9")
	seed("PAGE-2", "live", "")
	seed("PAGE-2", "review", "J-2")
	seed("PAGE-3", "live", "J-3")
	seed("PAGE-3", "review", "J-3")
	gate := setGate{"PAGE-1": true, "PAGE-2": true, "PAGE-3": true}
	policy, err := visibility.NewPolicyReader(gate, refRedactor{}, st)
	if err != nil {
		t.Fatal(err)
	}
	script, err := visibility.NewScriptReader(policy, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	reader := script.WithWorld(visibility.WorldOf(store.TrivialScope()))
	holders := []visibility.ExternalRefHolder{{Type: "page", Property: "jira"}}

	tests := []struct {
		ext  string
		want entity.Ref
	}{
		{ext: "J-1", want: entity.Ref{ID: "PAGE-1", Face: "live"}},
		{ext: "J-9", want: entity.Ref{ID: "PAGE-1", Face: "review"}},
		{ext: "J-2", want: entity.Ref{ID: "PAGE-2", Face: "review"}},
	}
	for _, tc := range tests {
		got, findErr := reader.FindByExternalRef(ctx, holders, tc.ext)
		if findErr != nil {
			t.Fatalf("%s: %v", tc.ext, findErr)
		}
		if len(got) != 1 || got[0].Ref() != tc.want {
			t.Errorf("%s: got %v, want %v", tc.ext, got, tc.want)
		}
	}
	got, err := reader.FindByExternalRef(ctx, holders, "J-3")
	if ids := foundIDs(t, got, err); !slices.Equal(ids, []string{"PAGE-3"}) {
		t.Errorf("faces sharing an id: got %v, want PAGE-3 once", ids)
	}
}
