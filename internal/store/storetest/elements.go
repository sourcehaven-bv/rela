package storetest

import (
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/canonical"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/reconcile"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// StoredElement is the reconciliation element a backend stores on one live
// row (TKT-1G2XR1R). Hash is "" when the row has none.
type StoredElement struct {
	Kind    reconcile.Kind
	Address string
	Hash    string
	Limbs   [4]int64
}

// StoredElementsFunc returns the stored element of every live entity and
// relation row of s, in any order. It is a test hook into the backend's own
// columns, not a store capability.
type StoredElementsFunc func(t *testing.T, s store.Store) []StoredElement

// RunElementTests pins that a backend that stores reconciliation elements
// keeps them equal to the elements computed from what it reads back, after
// every kind of write: create, update, an update that does not change the
// canonical hash, rename, endpoint swap, family replacement, soft delete and
// restore, and delete. A row without an element fails the check, because no
// live row may have a NULL hash after a write.
func RunElementTests(t *testing.T, f Factory, stored StoredElementsFunc, caps Capabilities) {
	s := f(t)
	check := func(step string) {
		t.Helper()
		RequireElementsMatch(t, s, stored, step)
	}

	for _, e := range []*entity.Entity{
		elementEntity("FEAT-001", "", "feature", "Login"),
		elementEntity("FEAT-001", "draft", "feature", "Login draft"),
		elementEntity("FEAT-002", "", "feature", "Ünïcode ✓"),
		elementEntity("REQ-001", "", "requirement", "Must authenticate"),
	} {
		require.NoError(t, s.CreateEntity(ctx(), e))
	}
	for _, k := range []entity.RelationKey{
		{From: "FEAT-001", Type: "implements", To: "REQ-001"},
		{From: "FEAT-001", FromFace: "draft", Type: "refines", To: "REQ-001"},
		{From: "FEAT-002", Type: "depends-on", To: "FEAT-001"},
		// A self-loop: a rename moves both of its ends at once.
		{From: "FEAT-001", Type: "depends-on", To: "FEAT-001"},
	} {
		_, err := s.CreateRelation(ctx(), k, &store.RelationData{
			Properties: map[string]any{"weight": 1.5, "note": "n"}, Content: "edge body",
		})
		require.NoError(t, err)
	}
	check("create")

	upd := elementEntity("FEAT-001", "", "feature", "Login v2")
	upd.Content = "new body"
	require.NoError(t, s.UpdateEntity(ctx(), upd))
	_, err := s.UpdateRelation(ctx(), entity.RelationKey{From: "FEAT-001", Type: "implements", To: "REQ-001"},
		store.RelationData{Properties: map[string]any{"weight": 2}, Content: "changed"})
	require.NoError(t, err)
	check("update")

	// Different stored text with the same canonical hash: the body is hashed
	// after markdown formatting. A backend whose trigger clears an element
	// when the stored text changes and the hash does not must refill it.
	same := upd.Clone()
	same.Content = "new body\n\n"
	require.Equal(t, canonical.HashEntity(*upd), canonical.HashEntity(*same), "test premise")
	require.NoError(t, s.UpdateEntity(ctx(), same))
	_, err = s.UpdateRelation(ctx(), entity.RelationKey{From: "FEAT-001", Type: "implements", To: "REQ-001"},
		store.RelationData{Properties: map[string]any{"weight": 2}, Content: "changed\n\n"})
	require.NoError(t, err)
	check("update with an unchanged hash")

	_, err = s.RenameFamily(ctx(), "FEAT-001", "FEAT-100")
	require.NoError(t, err)
	check("rename")

	_, err = store.SwapRelationEndpoints(ctx(), s, "implements")
	require.NoError(t, err)
	check("swap endpoints")

	if caps.FamilyReplace {
		conv := elementEntity("FEAT-002", "", "requirement", "Converted")
		require.NoError(t, familyReplacerOf(t, s).ReplaceEntityFamily(ctx(), "FEAT-002",
			[]store.FamilyReplacement{{SourceFace: "", Entity: conv}}))
		check("replace family")
	}

	if caps.SoftDelete {
		sd, ok := s.(store.SoftDeleteProvider)
		require.True(t, ok, "store declared Capabilities.SoftDelete but is not a store.SoftDeleteProvider")
		_, err = sd.SoftDelete().MarkDeleted(ctx(), "REQ-001", "tester")
		require.NoError(t, err)
		check("mark deleted")
		_, err = sd.SoftDelete().Unmark(ctx(), "REQ-001")
		require.NoError(t, err)
		check("restore")
	}

	_, err = s.DeleteFace(ctx(), entity.Ref{ID: "FEAT-100", Face: "draft"})
	require.NoError(t, err)
	check("delete face")
	_, err = s.DeleteFamily(ctx(), "FEAT-100", true)
	require.NoError(t, err)
	check("delete family")
}

func elementEntity(id string, face entity.Face, typ, title string) *entity.Entity {
	e := entity.New(id, typ)
	e.Face = face
	e.SetString("title", title)
	e.Properties["count"] = 3
	e.Properties["ratio"] = 0.25
	// Numbers whose JSON form differs between encoders: the stored hash must
	// still equal the hash of the value a read returns.
	e.Properties["big"] = 1e21
	e.Properties["small"] = 1e-7
	e.Properties["inexact"] = 0.30000000000000004
	e.Properties["tags"] = []any{"a", "b"}
	e.Content = "body of " + id
	return e
}

// RequireElementsMatch fails t unless the stored elements of s equal the elements
// computed from its live rows. step names the write in the failure message.
func RequireElementsMatch(t *testing.T, s store.Store, stored StoredElementsFunc, step string) {
	t.Helper()
	var want []StoredElement
	for e, err := range s.ListEntities(ctx(), store.EntityQuery{Faces: store.AllFaces()}) {
		require.NoError(t, err)
		want = append(want, storedOf(reconcile.EntityElement(*e)))
	}
	for r, err := range s.ListRelations(ctx(), store.RelationQuery{}) {
		require.NoError(t, err)
		want = append(want, storedOf(reconcile.RelationElement(*r)))
	}
	got := stored(t, s)
	sortElements(want)
	sortElements(got)
	require.Equal(t, want, got, "stored elements after %s", step)
}

func storedOf(el reconcile.Element) StoredElement {
	out := StoredElement{Kind: el.Kind, Address: el.Address, Hash: el.Hash.String()}
	for i, l := range el.Fingerprint().Limbs() {
		out.Limbs[i] = int64(l)
	}
	return out
}

func sortElements(els []StoredElement) {
	slices.SortFunc(els, func(a, b StoredElement) int {
		if a.Kind != b.Kind {
			return int(a.Kind) - int(b.Kind)
		}
		return strings.Compare(a.Address, b.Address)
	})
}
