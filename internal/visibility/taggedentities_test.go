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

// listedTags serves fixed tagged rows and no tagged snapshots.
type listedTags []store.TaggedEntity

func (listedTags) VersionByTag(context.Context, entity.Ref, store.VersionTagName) (*store.VersionSnapshot, error) {
	return nil, store.ErrNotFound
}

func (c listedTags) TaggedEntities(_ context.Context, _ store.VersionTagName, typ string) ([]store.TaggedEntity, error) {
	var out []store.TaggedEntity
	for _, row := range c {
		if typ == "" || row.Type == typ {
			out = append(out, row)
		}
	}
	return out, nil
}

// tagGate permits the ids in read, the faces in faces (nil: every face), and
// history:read when history is set.
type tagGate struct {
	read    map[string]bool
	faces   []entity.Face
	history bool
}

func (g tagGate) PermitsRead(_ context.Context, _, id string) (bool, error) { return g.read[id], nil }

func (g tagGate) ReadableFacesMany(_ context.Context, _ string, ids []string) (acl.FaceVerdicts, error) {
	out := make(map[string]bool, len(ids))
	for _, id := range ids {
		out[id] = g.read[id]
	}
	return visibilitytest.IDVerdicts(out, nil)
}

func (g tagGate) PermittedFaces(context.Context, string) ([]entity.Face, error) { return g.faces, nil }

func (g tagGate) HoldsPermission(_ context.Context, perm string) bool {
	return g.history && perm == acl.PermHistoryRead
}

// noPermGate is tagGate without the permission half.
type noPermGate struct{ tagGate }

func (g noPermGate) HoldsPermission() {}

func tagRow(id, typ string, deleted bool) store.TaggedEntity {
	return store.TaggedEntity{Ref: entity.Ref{ID: id}, Type: typ, Version: 1, Deleted: deleted}
}

func mustTag(t *testing.T) store.VersionTagName {
	t.Helper()
	n, err := store.ParseVersionTagName("baseline/audit-2026")
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// taggedFixture stores TKT-1, TKT-2 (ticket) and REQ-1 (requirement) and tags
// them, plus the deleted TKT-9@draft, a deleted REQ-1 lifecycle of another type, and
// a deleted TKT-2 lifecycle.
func taggedFixture(t *testing.T, gate visibility.RowGate) (*visibility.ScriptReader, listedTags) {
	t.Helper()
	st := memstore.New()
	for id, typ := range map[string]string{"TKT-1": "ticket", "TKT-2": "ticket", "REQ-1": "requirement"} {
		if err := st.CreateEntity(context.Background(), entity.New(id, typ)); err != nil {
			t.Fatal(err)
		}
	}
	policy, err := visibility.NewPolicyReader(gate, visibility.NopRedactor{}, st)
	if err != nil {
		t.Fatal(err)
	}
	script, err := visibility.NewScriptReader(policy, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	tags := listedTags{
		tagRow("REQ-1", "requirement", false),
		tagRow("REQ-1", "ticket", true),
		tagRow("TKT-1", "ticket", false),
		tagRow("TKT-2", "ticket", false),
		tagRow("TKT-2", "ticket", true),
		{Ref: entity.Ref{ID: "TKT-9", Face: "draft"}, Type: "ticket", Version: 1, Deleted: true},
	}
	return script.WithWorld(visibility.WorldOf(store.TrivialScope())).
		WithHistory(cannedHistory{}, tags), tags
}

func taggedIDs(rows []store.TaggedEntity) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.Ref.ID
		if r.Deleted {
			out[i] += "(deleted)"
		}
	}
	return out
}

func TestTaggedEntities_Gate(t *testing.T) {
	all := map[string]bool{"TKT-1": true, "TKT-2": true, "REQ-1": true}
	cases := []struct {
		name string
		gate visibility.RowGate
		typ  string
		want []string
	}{
		{"live only without history:read", tagGate{read: all}, "",
			[]string{"REQ-1", "TKT-1", "TKT-2"}},
		{"history:read adds deleted rows", tagGate{read: all, history: true}, "",
			[]string{"REQ-1", "TKT-1", "TKT-2", "TKT-2(deleted)", "TKT-9(deleted)"}},
		{"type filter", tagGate{read: all, history: true}, "ticket",
			[]string{"TKT-1", "TKT-2", "TKT-2(deleted)", "TKT-9(deleted)"}},
		{"hidden live row and its deleted lifecycle are left out", tagGate{read: map[string]bool{"TKT-1": true}, history: true}, "",
			[]string{"TKT-1", "TKT-9(deleted)"}},
		{"deleted face outside the face grant", tagGate{read: all, history: true, faces: []entity.Face{""}}, "",
			[]string{"REQ-1", "TKT-1", "TKT-2", "TKT-2(deleted)"}},
		{"gate without permissions fails closed", noPermGate{tagGate{read: all, history: true}}, "",
			[]string{"REQ-1", "TKT-1", "TKT-2"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, _ := taggedFixture(t, tc.gate)
			got, err := r.TaggedEntities(context.Background(), mustTag(t), tc.typ)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(taggedIDs(got), tc.want) {
				t.Errorf("got %v, want %v", taggedIDs(got), tc.want)
			}
		})
	}
}

func TestTaggedEntities_LiveTypeMismatchIsLeftOut(t *testing.T) {
	r, _ := taggedFixture(t, tagGate{read: map[string]bool{"TKT-1": true}})
	r = r.WithHistory(cannedHistory{}, listedTags{tagRow("TKT-1", "requirement", false)})
	got, err := r.TaggedEntities(context.Background(), mustTag(t), "")
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, %v; want nothing", got, err)
	}
}

func TestTaggedEntities_DeniedWorldListsNothing(t *testing.T) {
	r, _ := taggedFixture(t, tagGate{read: map[string]bool{"TKT-1": true}, history: true})
	got, err := r.WithWorld(visibility.DeniedWorld()).TaggedEntities(context.Background(), mustTag(t), "")
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, %v; want nothing", got, err)
	}
}

func TestTaggedEntities_Unsupported(t *testing.T) {
	r, _ := taggedFixture(t, tagGate{})
	ctx := context.Background()
	for name, reader := range map[string]interface {
		TaggedEntities(context.Context, store.VersionTagName, string) ([]store.TaggedEntity, error)
	}{
		"no history":           r.WithHistory(nil, nil),
		"no tags":              r.WithHistory(cannedHistory{}, nil),
		"unrestricted, none":   visibility.Unrestricted(memstore.New()),
		"unrestricted no tags": visibility.Unrestricted(memstore.New()).WithHistory(cannedHistory{}, nil),
	} {
		if _, err := reader.TaggedEntities(ctx, mustTag(t), ""); !errors.Is(err, store.ErrHistoryUnsupported) {
			t.Errorf("%s: err = %v, want ErrHistoryUnsupported", name, err)
		}
	}
}

func TestTaggedEntities_UnrestrictedAndDeny(t *testing.T) {
	_, tags := taggedFixture(t, tagGate{})
	ctx := context.Background()
	u := visibility.Unrestricted(memstore.New()).WithHistory(cannedHistory{}, tags)
	got, err := u.TaggedEntities(ctx, mustTag(t), "")
	if err != nil || len(got) != len(tags) {
		t.Fatalf("unrestricted: got %d rows, %v; want %d", len(got), err, len(tags))
	}
	if got, err := (visibility.DenyReader{}).TaggedEntities(ctx, mustTag(t), ""); err != nil || len(got) != 0 {
		t.Fatalf("deny: got %v, %v", got, err)
	}
}

// faceOnlyGate reads only the faces in read, of every id, and holds
// history:read.
type faceOnlyGate struct{ read []entity.Face }

func (faceOnlyGate) PermitsRead(context.Context, string, string) (bool, error) { return true, nil }

func (g faceOnlyGate) ReadableFacesMany(_ context.Context, _ string, ids []string) (acl.FaceVerdicts, error) {
	byID := make(map[string]acl.FaceVerdict, len(ids))
	for _, id := range ids {
		byID[id] = acl.FacesVerdict(g.read...)
	}
	return acl.PerEntityVerdicts(byID), nil
}

func (faceOnlyGate) HoldsPermission(_ context.Context, perm string) bool {
	return perm == acl.PermHistoryRead
}

// TestTaggedEntities_DeletedFaceLivingAgain: a deleted lifecycle of a face
// that lives again is listed only when the new row is readable, as the
// history endpoint serves that face's history as a live read.
func TestTaggedEntities_DeletedFaceLivingAgain(t *testing.T) {
	pub := entity.Ref{ID: "POL-1", Face: "pub"}
	for name, tc := range map[string]struct {
		read []entity.Face
		want int
	}{
		"reused face hidden, sibling readable": {read: []entity.Face{"draft"}, want: 0},
		"reused face readable":                 {read: []entity.Face{"draft", "pub"}, want: 1},
	} {
		t.Run(name, func(t *testing.T) {
			st := memstore.New()
			for _, f := range []entity.Face{"draft", "pub"} {
				if err := st.CreateEntity(context.Background(), &entity.Entity{ID: "POL-1", Type: "policy", Face: f}); err != nil {
					t.Fatal(err)
				}
			}
			policy, err := visibility.NewPolicyReader(faceOnlyGate{read: tc.read}, visibility.NopRedactor{}, st)
			if err != nil {
				t.Fatal(err)
			}
			script, err := visibility.NewScriptReader(policy, st, nil)
			if err != nil {
				t.Fatal(err)
			}
			r := script.WithWorld(visibility.WorldOf(store.TrivialScope())).WithHistory(cannedHistory{}, listedTags{{Ref: pub, Type: "policy", Version: 1, Deleted: true}})
			got, err := r.TaggedEntities(context.Background(), mustTag(t), "")
			if err != nil || len(got) != tc.want {
				t.Fatalf("got %v, %v; want %d rows", got, err, tc.want)
			}
		})
	}
}
