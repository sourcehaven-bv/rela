package storetest

import (
	"context"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/reconcile"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/elementnaive"
)

// ElementRanger is the element range reader of a backend (TKT-IQU106C); see
// store.ElementScope.
type ElementRanger interface {
	RangeFingerprint(ctx context.Context, scope store.ElementScope, r store.ElementRange) (store.RangeSum, error)
	ListRange(
		ctx context.Context, scope store.ElementScope, r store.ElementRange, l store.RangeList,
	) ([]store.RangeEntry, error)
}

// ElementRangesFunc returns the backend's element range reader over s.
type ElementRangesFunc func(t *testing.T, s store.Store) ElementRanger

// NaiveElementRanges is the [ElementRangesFunc] of a backend that answers
// through elementnaive: memstore and fsstore.
func NaiveElementRanges(t *testing.T, s store.Store) ElementRanger {
	t.Helper()
	r, err := elementnaive.New(s)
	require.NoError(t, err)
	return r
}

// The element range seed. Faces and rows:
//
//	doc/D1         level public, secret set, show yes -> folded
//	doc/D1@draft   level public, no secret            -> folded
//	doc/D2         level public, secret set, no show  -> pending
//	doc/D3         level private                      -> gated out
//	doc/D4         level public, secret held as null  -> pending
//	doc/D5@review  level public                       -> outside the face allowlist
//	note/N1                                           -> pending (unknown visibility)
//	user/U1, user/U2 (U2 stores a roll-up name)       -> folded
//	vault/V1                                          -> type not in scope
//
// Relations:
//
//	U1 --owns--> D1               in, folded
//	U1 --owns--> D3               head gated out
//	U2 --owns--> V1               head type not in scope
//	D1@draft --links--> U2        faced tail admitted, pending (links has unknown visibility)
//	D5@review --links--> U1       faced tail outside the face allowlist
//	D3 --links--> U1              tail gated out
//	N1 --links--> D2              identity tail, pending
//	U1 --cites--> D1              relation type not in scope
func seedElementRange(t *testing.T, s store.Store) {
	t.Helper()
	add := func(typ, id string, face entity.Face, props map[string]any) {
		e := entity.New(id, typ)
		e.Face = face
		e.Properties = props
		e.Content = "body of " + id
		require.NoError(t, s.CreateEntity(ctx(), e), "create %s/%s@%s", typ, id, face)
	}
	add("doc", "D1", "", map[string]any{"level": "public", "secret": "s1", "show": "yes"})
	add("doc", "D1", "draft", map[string]any{"level": "public"})
	add("doc", "D2", "", map[string]any{"level": "public", "secret": "s2"})
	add("doc", "D3", "", map[string]any{"level": "private", "secret": "s3", "show": "yes"})
	add("doc", "D4", "", map[string]any{"level": "public", "secret": nil})
	add("doc", "D5", "review", map[string]any{"level": "public"})
	add("note", "N1", "", map[string]any{"text": "n"})
	add("user", "U1", "", map[string]any{"name": "one"})
	add("user", "U2", "", map[string]any{"name": "two", "total": 5})
	add("vault", "V1", "", map[string]any{"key": "k"})
	for _, k := range []entity.RelationKey{
		{From: "U1", Type: "owns", To: "D1"},
		{From: "U1", Type: "owns", To: "D3"},
		{From: "U2", Type: "owns", To: "V1"},
		{From: "D1", FromFace: "draft", Type: "links", To: "U2"},
		{From: "D5", FromFace: "review", Type: "links", To: "U1"},
		{From: "D3", Type: "links", To: "U1"},
		{From: "N1", Type: "links", To: "D2"},
		{From: "U1", Type: "cites", To: "D1"},
	} {
		_, err := s.CreateRelation(ctx(), k, &store.RelationData{Properties: map[string]any{"w": 1}, Content: "edge"})
		require.NoError(t, err, "relation %s", k)
	}
}

// elementRangeScope is the reader of the seed: docs gated on level public at
// the default and draft faces with secret visible where show is yes, notes
// with unknown visibility, users with every field visible, owns folded and
// links of unknown visibility.
func elementRangeScope() store.ElementScope {
	eq := func(p, v string) store.PropPredicate {
		return store.PropPredicate{Property: p, Op: store.PropEqual, Value: v, Scalar: true}
	}
	return store.ElementScope{
		Entities: map[string]store.ElementTypeScope{
			"doc": {
				Query: &store.GraphQuery{Props: []store.PropPredicate{eq("level", "public")}},
				Faces: []entity.Face{"", "draft"},
				Visibility: &store.FieldVisibility{Hidden: []store.HiddenField{{
					Property: "secret",
					Visible:  store.RowCondition{Any: []*store.GraphQuery{{Props: []store.PropPredicate{eq("show", "yes")}}}},
				}}},
			},
			"note": {AllowAll: true},
			"user": {AllowAll: true, Visibility: &store.FieldVisibility{}},
		},
		Relations: map[string]store.RelationElementScope{
			"owns":  {Visibility: &store.FieldVisibility{}},
			"links": {},
		},
	}
}

// wantElement is one expected element: its address and whether it is pending.
type wantElement struct {
	addr    string
	pending bool
}

// RunElementRangeTests is the conformance suite for the element range read
// (store.ElementScope, TKT-IQU106C). Every expectation is computed from the
// rows the store reads back, with the reconcile package, so a backend's
// stored columns must equal the elements of what it serves.
//
// Run via [RunAll].
func RunElementRangeTests(t *testing.T, f Factory, ranges ElementRangesFunc) {
	t.Helper()
	s := f(t)
	seedElementRange(t, s)
	g := ranges(t, s)
	elements := readBackElements(t, s)
	scope := elementRangeScope()
	all := store.ElementRange{Unbounded: true}
	allRel := store.ElementRange{Relations: true, Unbounded: true}

	wantEntities := []wantElement{
		{"doc/D1", false}, {"doc/D1@draft", false}, {"doc/D2", true}, {"doc/D4", true},
		{"note/N1", true}, {"user/U1", false}, {"user/U2", false},
	}
	docs, fromNote := wantEntities[:4], wantEntities[4:]
	wantRelations := []wantElement{
		{"D1@draft--links--U2", true}, {"N1--links--D2", true}, {"U1--owns--D1", false},
	}

	t.Run("entities", func(t *testing.T) {
		requireRange(t, g, elements, scope, all, wantEntities)
	})
	t.Run("relations_need_both_endpoints", func(t *testing.T) {
		requireRange(t, g, elements, scope, allRel, wantRelations)
	})
	t.Run("bounds", func(t *testing.T) {
		requireRange(t, g, elements, scope,
			store.ElementRange{Lo: "doc/D1@draft", Hi: "doc/D2"}, wantEntities[1:2])
		requireRange(t, g, elements, scope,
			store.ElementRange{Lo: "doc/D1", Hi: "doc/D1@draft"}, wantEntities[:1])
		requireRange(t, g, elements, scope, store.ElementRange{Lo: "note/N1", Unbounded: true}, fromNote)
		requireRange(t, g, elements, scope, store.ElementRange{Lo: "doc/", Hi: "doc0"}, docs)
		requireRange(t, g, elements, scope,
			store.ElementRange{Relations: true, Lo: "N", Hi: "U"}, wantRelations[1:2])
	})
	t.Run("empty_ranges", func(t *testing.T) {
		requireRange(t, g, elements, scope, store.ElementRange{}, nil)
		requireRange(t, g, elements, scope, store.ElementRange{Lo: "doc/D1", Hi: "doc/D1"}, nil)
		requireRange(t, g, elements, scope, store.ElementRange{Lo: "zzz", Unbounded: true}, nil)
		requireRange(t, g, elements, store.ElementScope{}, all, nil)
		requireRange(t, g, elements, store.ElementScope{}, allRel, nil)
	})
	t.Run("ranges_partition", func(t *testing.T) {
		left := rangeSum(t, g, scope, store.ElementRange{Hi: "doc/D2"})
		right := rangeSum(t, g, scope, store.ElementRange{Lo: "doc/D2", Unbounded: true})
		whole := rangeSum(t, g, scope, all)
		require.Equal(t, whole.Count, left.Count+right.Count)
		require.Equal(t, whole.Pending, left.Pending+right.Pending)
		for i := range whole.LimbSums {
			require.Equal(t, whole.LimbSums[i], left.LimbSums[i]+right.LimbSums[i])
		}
	})
	t.Run("list_limit_and_pending_only", func(t *testing.T) {
		got, err := g.ListRange(ctx(), scope, all, store.RangeList{Limit: 2})
		require.NoError(t, err)
		require.Equal(t, []string{"doc/D1", "doc/D1@draft"}, entryAddresses(got))
		got, err = g.ListRange(ctx(), scope, all, store.RangeList{Limit: 10, PendingOnly: true})
		require.NoError(t, err)
		require.Equal(t, []string{"doc/D2", "doc/D4", "note/N1"}, entryAddresses(got))
		got, err = g.ListRange(ctx(), scope, allRel, store.RangeList{Limit: 1, PendingOnly: true})
		require.NoError(t, err)
		require.Equal(t, []string{"D1@draft--links--U2"}, entryAddresses(got))
	})
	t.Run("unknown_visibility_is_pending", func(t *testing.T) {
		sc := elementRangeScope()
		doc := sc.Entities["doc"]
		doc.Visibility = nil
		sc.Entities["doc"] = doc
		sc.Relations["owns"] = store.RelationElementScope{}
		requireRange(t, g, elements, sc, all, []wantElement{
			{"doc/D1", true}, {"doc/D1@draft", true}, {"doc/D2", true}, {"doc/D4", true},
			{"note/N1", true}, {"user/U1", false}, {"user/U2", false},
		})
		requireRange(t, g, elements, sc, allRel, []wantElement{
			{"D1@draft--links--U2", true}, {"N1--links--D2", true}, {"U1--owns--D1", true},
		})
	})
	t.Run("rollup_name_with_no_condition_is_pending", func(t *testing.T) {
		sc := elementRangeScope()
		sc.Entities["user"] = store.ElementTypeScope{AllowAll: true, Visibility: &store.FieldVisibility{
			Hidden: []store.HiddenField{{Property: "total"}},
		}}
		requireRange(t, g, elements, sc, store.ElementRange{Lo: "user/", Hi: "user0"}, []wantElement{
			{"user/U1", false}, {"user/U2", true},
		})
	})
	t.Run("relation_gate", func(t *testing.T) {
		sc := elementRangeScope()
		sc.Entities["doc"] = store.ElementTypeScope{
			Query:      &store.GraphQuery{HasInbound: &store.RelationPredicate{OfTypes: []string{"owns"}}},
			Visibility: &store.FieldVisibility{},
		}
		requireRange(t, g, elements, sc, store.ElementRange{Lo: "doc/", Hi: "doc0"}, []wantElement{
			{"doc/D1", false}, {"doc/D1@draft", false}, {"doc/D3", false},
		})
	})
	t.Run("gate_template_is_not_changed", func(t *testing.T) {
		sc := elementRangeScope()
		before := *sc.Entities["doc"].Query
		_ = rangeSum(t, g, sc, all)
		_, err := g.ListRange(ctx(), sc, allRel, store.RangeList{Limit: 5})
		require.NoError(t, err)
		require.Equal(t, before, *sc.Entities["doc"].Query)
	})
	t.Run("invalid_requests", func(t *testing.T) {
		RunElementRangeValidation(t, g)
	})
}

// RunElementRangeValidation checks that g refuses every malformed request
// with store.ErrInvalidQuery.
func RunElementRangeValidation(t *testing.T, g ElementRanger) {
	t.Helper()
	q := &store.GraphQuery{}
	hidden := func(whens ...*store.GraphQuery) store.ElementScope {
		return docScope(store.ElementTypeScope{AllowAll: true, Visibility: &store.FieldVisibility{
			Hidden: []store.HiddenField{{Property: "secret", Visible: store.RowCondition{Any: whens}}},
		}})
	}
	badOp := &store.GraphQuery{Props: []store.PropPredicate{{Property: "show", Op: store.PropOp(99)}}}
	// Every scope case uses a range that admits rows, so a reader that
	// checks lazily, only for the rows it reads, fails too.
	all := store.ElementRange{Unbounded: true}
	scopeCases := map[string]store.ElementScope{
		"wildcard_key":    {Entities: map[string]store.ElementTypeScope{"*": {AllowAll: true}}},
		"empty_key":       {Relations: map[string]store.RelationElementScope{"": {}}},
		"type_mismatch":   docScope(store.ElementTypeScope{Query: &store.GraphQuery{EntityType: "user"}}),
		"allow_and_query": docScope(store.ElementTypeScope{AllowAll: true, Query: q}),
		"empty_faces":     docScope(store.ElementTypeScope{AllowAll: true, Faces: []entity.Face{}}),
		"empty_property": docScope(store.ElementTypeScope{AllowAll: true,
			Visibility: &store.FieldVisibility{Hidden: []store.HiddenField{{}}}}),
		"relation_hidden": {Relations: map[string]store.RelationElementScope{
			"owns": {Visibility: &store.FieldVisibility{Hidden: []store.HiddenField{{Property: "w"}}}}}},
		"gate_bad_shape":         docScope(store.ElementTypeScope{Query: badOp}),
		"hidden_bad_shape":       hidden(badOp),
		"hidden_type_mismatch":   hidden(&store.GraphQuery{EntityType: "user"}),
		"hidden_bad_shape_later": hidden(nil, badOp), // nil is valid; the next one is not
	}
	for name, scope := range scopeCases {
		t.Run(name, func(t *testing.T) {
			requireInvalid(t, g, scope, all, store.RangeList{Limit: 10})
		})
	}
	rangeCases := map[string]store.ElementRange{
		"lo_after_hi":    {Lo: "b", Hi: "a"},
		"bound_too_long": {Lo: strings.Repeat("a", store.MaxRangeBound+1), Unbounded: true},
		"bound_not_utf8": {Lo: "doc/\xff", Unbounded: true},
		"bound_nul":      {Lo: "doc/\x00", Unbounded: true},
	}
	for name, r := range rangeCases {
		t.Run(name, func(t *testing.T) {
			requireInvalid(t, g, elementRangeScope(), r, store.RangeList{Limit: 10})
		})
	}
	for name, limit := range map[string]int{"limit_zero": 0, "limit_too_big": store.MaxRangeList + 1} {
		t.Run(name, func(t *testing.T) {
			_, err := g.ListRange(ctx(), elementRangeScope(), all, store.RangeList{Limit: limit})
			require.ErrorIs(t, err, store.ErrInvalidQuery)
		})
	}
}

// requireInvalid checks that both reads refuse the request.
func requireInvalid(t *testing.T, g ElementRanger, scope store.ElementScope, r store.ElementRange, l store.RangeList) {
	t.Helper()
	_, err := g.RangeFingerprint(ctx(), scope, r)
	require.ErrorIs(t, err, store.ErrInvalidQuery, "RangeFingerprint")
	_, err = g.ListRange(ctx(), scope, r, l)
	require.ErrorIs(t, err, store.ErrInvalidQuery, "ListRange")
}

func docScope(ts store.ElementTypeScope) store.ElementScope {
	return store.ElementScope{Entities: map[string]store.ElementTypeScope{"doc": ts}}
}

// readBackElements returns the element of every row s serves, by address.
func readBackElements(t *testing.T, s store.Store) map[string]reconcile.Element {
	t.Helper()
	out := map[string]reconcile.Element{}
	for e, err := range s.ListEntities(ctx(), store.EntityQuery{Faces: store.AllFaces()}) {
		require.NoError(t, err)
		el := reconcile.EntityElement(*e)
		out[el.Address] = el
	}
	for r, err := range s.ListRelations(ctx(), store.RelationQuery{}) {
		require.NoError(t, err)
		el := reconcile.RelationElement(*r)
		out[el.Address] = el
	}
	return out
}

// requireRange checks both reads of r against want.
func requireRange(
	t *testing.T, g ElementRanger, elements map[string]reconcile.Element,
	scope store.ElementScope, r store.ElementRange, want []wantElement,
) {
	t.Helper()
	var wantSum store.RangeSum
	var wantList []store.RangeEntry
	for _, w := range want {
		el, ok := elements[w.addr]
		require.True(t, ok, "test premise: no row at %s", w.addr)
		wantSum.Count++
		entry := store.RangeEntry{Address: w.addr, Pending: w.pending}
		if w.pending {
			wantSum.Pending++
		} else {
			entry.Hash = el.Hash.String()
			for i, l := range el.Columns().Limbs {
				wantSum.LimbSums[i] += uint64(l)
			}
		}
		wantList = append(wantList, entry)
	}
	require.Equal(t, wantSum, rangeSum(t, g, scope, r), "RangeFingerprint %+v", r)

	got, err := g.ListRange(ctx(), scope, r, store.RangeList{Limit: store.MaxRangeList})
	require.NoError(t, err)
	stripped := make([]store.RangeEntry, len(got))
	for i, e := range got {
		requireEntryKey(t, e, r.Relations)
		stripped[i] = store.RangeEntry{Address: e.Address, Hash: e.Hash, Pending: e.Pending}
	}
	if wantList == nil {
		wantList = []store.RangeEntry{}
	}
	require.Equal(t, wantList, append([]store.RangeEntry{}, stripped...), "ListRange %+v", r)
}

// requireEntryKey checks that an entry's key names its address.
func requireEntryKey(t *testing.T, e store.RangeEntry, relations bool) {
	t.Helper()
	if relations {
		require.Equal(t, e.Address, e.Relation.String())
		return
	}
	require.Equal(t, e.Address, reconcile.EntityAddress(e.Type, e.Ref))
}

func rangeSum(t *testing.T, g ElementRanger, scope store.ElementScope, r store.ElementRange) store.RangeSum {
	t.Helper()
	sum, err := g.RangeFingerprint(ctx(), scope, r)
	require.NoError(t, err)
	return sum
}

func entryAddresses(es []store.RangeEntry) []string {
	out := make([]string, len(es))
	for i, e := range es {
		out[i] = e.Address
	}
	return out
}

// elementDiffCases is how many generated requests
// [RunElementRangeDifferential] checks.
const elementDiffCases = 300

// RunElementRangeDifferential compares a backend's element range reader with
// elementnaive over the SAME store, on scopes and ranges generated from
// fixed seeds over the graph differential's seed (faces, awkward property
// names, relation-gated types, faced tails). Both read the same rows, so a
// difference is in the gate, the hidden-field test or the fold, not in how
// the store represents a value: RunElementTests pins that the stored columns
// equal the elements of the rows read back, and RunGraphDifferential pins
// the store's MatchingFaces against graphquerynaive.
//
// The cases must exercise something: at least a quarter must admit rows and
// a tenth must mix folded and pending rows. native, when not nil, reports
// whether the backend answers a request in SQL rather than through its
// elementnaive fallback, which compares the reference with itself; at least
// half the cases must be native.
func RunElementRangeDifferential(
	t *testing.T, f Factory, ranges ElementRangesFunc, native func(store.ElementScope, store.ElementRange) bool,
) {
	s := f(t)
	seedGraphDiff(t, s)
	g := ranges(t, s)
	ref := NaiveElementRanges(t, s)
	addrs := slices.Sorted(func(yield func(string) bool) {
		for a := range readBackElements(t, s) {
			if !yield(a) {
				return
			}
		}
	})

	fails, admitting, mixed, natives := 0, 0, 0, 0
	for i := range elementDiffCases {
		rng := rand.New(rand.NewPCG(uint64(i), 0xE1E7))
		scope := genElementScope(rng)
		r := genElementRange(rng, addrs)
		l := store.RangeList{Limit: 1 + rng.IntN(40), PendingOnly: rng.IntN(4) == 0}
		if native == nil || native(scope, r) {
			natives++
		}
		want, msg := diffElementRange(g, ref, scope, r, l)
		if want.Count > 0 {
			admitting++
		}
		if want.Pending > 0 && want.Pending < want.Count {
			mixed++
		}
		if msg != "" {
			fails++
			t.Errorf("case %d: %s\nrange: %+v", i, msg, r)
			if fails >= 20 {
				t.Fatal("too many differences; stopping")
			}
		}
	}
	require.GreaterOrEqual(t, admitting, elementDiffCases/4, "cases that admit rows")
	require.GreaterOrEqual(t, mixed, elementDiffCases/10, "cases with folded and pending rows")
	require.GreaterOrEqual(t, natives, elementDiffCases/2, "cases the backend answers in SQL")
}

// diffElementRange compares both reads and returns the reference's sum and
// a description of the first difference, or "".
func diffElementRange(
	g, ref ElementRanger, scope store.ElementScope, r store.ElementRange, l store.RangeList,
) (store.RangeSum, string) {
	want, wantErr := ref.RangeFingerprint(ctx(), scope, r)
	got, gotErr := g.RangeFingerprint(ctx(), scope, r)
	if (wantErr == nil) != (gotErr == nil) {
		return want, fmt.Sprintf("RangeFingerprint error: naive %v, store %v", wantErr, gotErr)
	}
	if want != got {
		return want, fmt.Sprintf("RangeFingerprint: naive %+v, store %+v", want, got)
	}
	return want, diffElementList(g, ref, scope, r, l)
}

func diffElementList(g, ref ElementRanger, scope store.ElementScope, r store.ElementRange, l store.RangeList) string {
	wantList, wantErr := ref.ListRange(ctx(), scope, r, l)
	gotList, gotErr := g.ListRange(ctx(), scope, r, l)
	if (wantErr == nil) != (gotErr == nil) {
		return fmt.Sprintf("ListRange error: naive %v, store %v", wantErr, gotErr)
	}
	if len(wantList) != len(gotList) {
		return fmt.Sprintf("ListRange: naive %v, store %v", entryAddresses(wantList), entryAddresses(gotList))
	}
	for i := range wantList {
		if wantList[i] != gotList[i] {
			return fmt.Sprintf("ListRange entry %d: naive %+v, store %+v", i, wantList[i], gotList[i])
		}
	}
	return ""
}

// genElementScope draws a scope over the graph differential's seed.
func genElementScope(rng *rand.Rand) store.ElementScope {
	scope := store.ElementScope{
		Entities:  map[string]store.ElementTypeScope{},
		Relations: map[string]store.RelationElementScope{},
	}
	for _, typ := range []string{"item", "group"} {
		var ts store.ElementTypeScope
		switch rng.IntN(5) {
		case 0:
			continue // denied
		case 1, 2:
			ts.AllowAll = true
		default:
			q := store.GraphQuery{}
			if typ == "item" {
				q = genGraphQuery(rng)
				q.EntityType = ""
			} else {
				q.Props = []store.PropPredicate{{
					Property: "status", Op: store.PropEqual, Value: pick(rng, []string{"open", "closed"}), Scalar: true,
				}}
			}
			ts.Query = &q
		}
		if rng.IntN(3) == 0 {
			ts.Faces = pickSome(rng, []entity.Face{"", "draft", "published"}, 3)
			if len(ts.Faces) == 0 {
				ts.Faces = []entity.Face{""}
			}
		}
		switch rng.IntN(3) {
		case 0: // unknown visibility
		case 1:
			ts.Visibility = &store.FieldVisibility{}
		default:
			ts.Visibility = &store.FieldVisibility{Hidden: genHiddenFields(rng)}
		}
		scope.Entities[typ] = ts
	}
	for _, rel := range []string{"partOf", "implements", "tracks", "childOf", "owns"} {
		switch rng.IntN(3) {
		case 0:
		case 1:
			scope.Relations[rel] = store.RelationElementScope{}
		default:
			scope.Relations[rel] = store.RelationElementScope{Visibility: &store.FieldVisibility{}}
		}
	}
	return scope
}

func genHiddenFields(rng *rand.Rand) []store.HiddenField {
	props := pickSome(rng, append(slices.Clone(graphDiffProps), "missing"), 3)
	out := make([]store.HiddenField, 0, len(props))
	for _, p := range props {
		h := store.HiddenField{Property: p}
		for range rng.IntN(3) {
			if rng.IntN(4) == 0 {
				h.Visible.Any = append(h.Visible.Any, nil)
				continue
			}
			q := genGraphQuery(rng)
			q.EntityType = ""
			h.Visible.Any = append(h.Visible.Any, &q)
		}
		out = append(out, h)
	}
	return out
}

// genElementRange draws a range of either kind, bounded by seeded addresses
// or by strings between them.
func genElementRange(rng *rand.Rand, addrs []string) store.ElementRange {
	bound := func() string {
		switch rng.IntN(4) {
		case 0:
			return ""
		case 1:
			return pick(rng, []string{"group/", "item/", "item/ITM-1", "GRP-", "ITM-0", "ITM-1", "zz"})
		default:
			return pick(rng, addrs)
		}
	}
	r := store.ElementRange{Relations: rng.IntN(2) == 0, Lo: bound(), Hi: bound(), Unbounded: rng.IntN(3) == 0}
	if !r.Unbounded && r.Lo > r.Hi {
		r.Lo, r.Hi = r.Hi, r.Lo
	}
	return r
}
