package visibility_test

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/reconcile"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/elementnaive"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// scopeSpy records every scope Elements sends to the store.
type scopeSpy struct {
	visibility.ElementRangeReader
	scopes []store.ElementScope
}

func (s *scopeSpy) RangeFingerprint(
	ctx context.Context, scope store.ElementScope, r store.ElementRange,
) (store.RangeSum, error) {
	s.scopes = append(s.scopes, scope)
	return s.ElementRangeReader.RangeFingerprint(ctx, scope, r)
}

func (s *scopeSpy) ListRange(
	ctx context.Context, scope store.ElementScope, r store.ElementRange, l store.RangeList,
) ([]store.RangeEntry, error) {
	s.scopes = append(s.scopes, scope)
	return s.ElementRangeReader.ListRange(ctx, scope, r, l)
}

// The store is never asked for a per-row verdict on a hidden field: a type
// either folds every row (empty Visibility) or none (nil Visibility). The
// store's answer to a per-row question would say which rows hold a hidden
// value.
func TestElements_NeverAsksStoreWhichRowsHoldHiddenValues(t *testing.T) {
	f := newElementsFixture(t, memstore.New(), 3, elementsPolicy())
	naive, err := elementnaive.New(f.st)
	if err != nil {
		t.Fatal(err)
	}
	spy := &scopeSpy{ElementRangeReader: naive}
	x := f.elements(spy, nil, 0)
	for _, user := range []string{"alice", "root", "bob"} {
		if _, err := x.Fingerprint(f.as(user), ticketTypes, allEntities); err != nil {
			t.Fatal(err)
		}
		if _, err := x.List(f.as(user), ticketTypes, allEntities, 10); err != nil {
			t.Fatal(err)
		}
	}
	if len(spy.scopes) == 0 {
		t.Fatal("no scope reached the store")
	}
	sawNil := false
	for _, sc := range spy.scopes {
		for typ, ts := range sc.Entities {
			if ts.Visibility == nil {
				sawNil = true
				continue
			}
			if len(ts.Visibility.Hidden) > 0 {
				t.Errorf("%s: per-row hidden fields sent to the store: %+v", typ, ts.Visibility.Hidden)
			}
		}
	}
	if !sawNil {
		t.Error("alice's ticket scope folded stored rows, though she has a hidden field")
	}
}

// staleRanges reports chosen rows of a stored type as not filled, and one
// with a stored hash that does not parse: what a database backend returns
// while it backfills, or after a bad write.
type staleRanges struct {
	visibility.ElementRangeReader
	pending map[string]bool
	corrupt string
}

func (s staleRanges) RangeFingerprint(
	ctx context.Context, scope store.ElementScope, r store.ElementRange,
) (store.RangeSum, error) {
	sum, err := s.ElementRangeReader.RangeFingerprint(ctx, scope, r)
	sum.Pending = uint64(len(s.pending)) // the sum is then unusable; Elements must list
	sum.LimbSums = [4]uint64{}
	return sum, err
}

func (s staleRanges) ListRange(
	ctx context.Context, scope store.ElementScope, r store.ElementRange, l store.RangeList,
) ([]store.RangeEntry, error) {
	out, err := s.ElementRangeReader.ListRange(ctx, scope, r, l)
	for i := range out {
		switch {
		case s.pending[out[i].Address]:
			out[i].Pending, out[i].Hash = true, ""
		case out[i].Address == s.corrupt:
			out[i].Hash = "not-a-digest"
		}
	}
	return out, err
}

// Rows of a stored type whose element is not filled, or does not parse,
// are recomputed, and the answer equals the filled one.
func TestElements_StoredTypeWithUnfilledRows(t *testing.T) {
	f := newElementsFixture(t, memstore.New(), 4, elementsPolicy())
	naive, err := elementnaive.New(f.st)
	if err != nil {
		t.Fatal(err)
	}
	root := f.as("root")
	filled := f.elements(nil, nil, 0)

	cases := []struct {
		name    string
		types   visibility.ElementTypes
		r       store.ElementRange
		pending []string
		corrupt string
	}{
		{
			name: "entities", types: ticketTypes, r: allEntities,
			pending: []string{"ticket/TKT-001", "person/bob"}, corrupt: "ticket/TKT-003",
		},
		{
			name: "relations",
			types: visibility.ElementTypes{
				Entities: []string{"person", "ticket"}, Relations: []string{"assigned-to"},
			},
			r:       store.ElementRange{Relations: true, Unbounded: true},
			pending: []string{relAddress("alice", "TKT-000")}, corrupt: relAddress("bob", "TKT-000"),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stale := staleRanges{ElementRangeReader: naive, pending: map[string]bool{}, corrupt: tc.corrupt}
			for _, a := range tc.pending {
				stale.pending[a] = true
			}
			x := f.elements(stale, nil, 0)

			want := fingerprint(root, t, filled, tc.types, tc.r)
			if got := fingerprint(root, t, x, tc.types, tc.r); got != want || got.Count == 0 {
				t.Errorf("Fingerprint = %v, want %v", got, want)
			}
			wantPage, err := filled.List(root, tc.types, tc.r, 10)
			if err != nil {
				t.Fatal(err)
			}
			gotPage, err := x.List(root, tc.types, tc.r, 10)
			if err != nil {
				t.Fatal(err)
			}
			if !equalElements(gotPage.Elements, wantPage.Elements) {
				t.Errorf("List = %v, want %v", gotPage.Elements, wantPage.Elements)
			}
		})
	}
}

func relAddress(from, to string) string {
	return reconcile.RelationElement(entity.Relation{From: from, Type: "assigned-to", To: to}).Address
}

func equalElements(a, b []reconcile.Element) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// faceReads admits every row of every type, limited to faces when set.
type faceReads struct{ faces []entity.Face }

func (r faceReads) ReadQueryFor(context.Context, string) (acl.ReadQueryResult, error) {
	return acl.ReadQueryResult{AllowAll: true, Faces: r.faces}, nil
}

func (faceReads) Bind(ctx context.Context) (context.Context, error) { return ctx, nil }

// A faced type: the face allowlist limits the rows, and the stored path and
// the recomputed path agree on every face's address.
func TestElements_Faces(t *testing.T) {
	ctx := context.Background()
	st := memstore.New()
	for _, id := range []string{"POL-1", "POL-2"} {
		for _, face := range []entity.Face{"draft", "published"} {
			e := &entity.Entity{
				ID: id, Type: "policy", Face: face, Content: string(face) + " body of " + id,
				Properties: map[string]any{"title": id + " " + string(face)},
			}
			if err := st.CreateEntity(ctx, e); err != nil {
				t.Fatal(err)
			}
		}
	}
	naive, err := elementnaive.New(st)
	if err != nil {
		t.Fatal(err)
	}
	build := func(faces []entity.Face, rollups rollupNames) *visibility.Elements {
		x, err := visibility.NewElements(visibility.ElementDeps{
			Ranges: naive, Rows: st, Reads: faceReads{faces: faces},
			Policy: visibility.NopRedactor{}, Rollups: rollups,
		})
		if err != nil {
			t.Fatal(err)
		}
		return x
	}
	types := visibility.ElementTypes{Entities: []string{"policy"}}
	for _, tc := range []struct {
		name  string
		faces []entity.Face
		count uint64
	}{
		{"every face", nil, 4},
		{"published only", []entity.Face{"published"}, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stored := fingerprint(ctx, t, build(tc.faces, rollupNames{}), types, allEntities)
			inGo := fingerprint(ctx, t, build(tc.faces, rollupNames{"policy": {"progress"}}), types, allEntities)
			if stored.Count != tc.count || stored != inGo {
				t.Errorf("stored path %v, Go path %v, want count %d on both", stored, inGo, tc.count)
			}
		})
	}
}
