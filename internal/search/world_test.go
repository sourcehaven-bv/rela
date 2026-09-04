package search_test

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// World-scoped search (TKT-9KZGJO). A world IS the search scope: searching a
// world means searching each entity's resolved prime in it, so the world's
// own chain answers "which face do I look at" and no per-world `searchable:`
// config is needed.
//
// These run against LinearSearch, which is the ground-truth backend the
// conformance suite holds the others to.

// publishedThenDraft is the ISMS demo world: prefer the published face, fall
// back to the draft, exclude anything with neither.
func publishedThenDraft() store.WorldScope {
	return store.NewWorldScope(map[string]store.TypeResolution{
		"policy": {
			Chain:    []entity.Face{"published", "draft"},
			Fallback: store.FallbackExclude,
		},
	})
}

func face(t *testing.T, l *search.LinearSearch, id string, p entity.Face, content string) {
	t.Helper()
	e := entity.New(id, "policy")
	e.Face = p
	e.Content = content
	if err := l.EntityPut(e); err != nil {
		t.Fatalf("EntityPut(%s@%s): %v", id, p, err)
	}
}

func facesByID(faces []search.Face) map[string]search.Face {
	out := make(map[string]search.Face, len(faces))
	for _, f := range faces {
		out[f.ID] = f
	}
	return out
}

// TestSearch_MatchesThePrimeNotTheFamily is the core semantic: a world
// searches each entity's RESOLVED face, not every face it happens to hold.
//
// POL-1's draft contains "sasquatch" and its published face does not. A
// `published`-world search for that term must find NOTHING: returning POL-1
// would show the reader published bytes that do not contain what they
// searched for — the mismatch between what was searched and what is
// displayed that world-scoped search exists to close.
func TestSearch_MatchesThePrimeNotTheFamily(t *testing.T) {
	l := search.NewLinearSearch()
	face(t, l, "POL-1", "published", "approved text")
	face(t, l, "POL-1", "draft", "sasquatch")

	faces, err := l.Search("sasquatch", 0, publishedThenDraft())
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(faces) != 0 {
		t.Errorf("got %v, want no results — the term exists only in the DRAFT, "+
			"and this world serves POL-1's published face. Matching the family "+
			"would show published bytes that lack the search term", faces)
	}

	// Control: the same index, same world, a term that IS in the prime.
	// Without this the assertion above passes against a search that returns
	// nothing at all.
	got, err := l.Search("approved", 0, publishedThenDraft())
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 1 || got[0].ID != "POL-1" {
		t.Fatalf("control: matching the PRIME's own text must hit; got %v", got)
	}
}

// TestSearch_ChainFallbackIsLabelled is the labeling requirement, and the
// half that the `via` gap made impossible until it was fixed.
//
// POL-2 has no published face, so the `published` world falls to its DRAFT.
// That is a legitimate hit — the design says a draft-only policy should be
// findable in the published world via its draft face — but the result must
// SAY it is a substitute, or the reader sees a hit whose displayed text is
// not the published text they think they are reading.
func TestSearch_ChainFallbackIsLabelled(t *testing.T) {
	l := search.NewLinearSearch()
	face(t, l, "POL-1", "published", "shared term")
	face(t, l, "POL-1", "draft", "shared term")
	face(t, l, "POL-2", "draft", "shared term")

	faces, err := l.Search("shared", 0, publishedThenDraft())
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	byID := facesByID(faces)
	if len(faces) != 2 {
		t.Fatalf("both policies match; got %d (%v)", len(faces), faces)
	}

	hit, ok := byID["POL-1"]
	if !ok {
		t.Fatal("POL-1 has a published face and must hit")
	}
	if hit.Face != entity.Face("published") || hit.ChainPosition != 0 {
		t.Errorf("POL-1 resolved to %q at position %d, want published at 0 — "+
			"the world's FIRST choice", hit.Face, hit.ChainPosition)
	}

	fell, ok := byID["POL-2"]
	if !ok {
		t.Fatal("POL-2 must still be findable via its draft face — a draft-only " +
			"policy IS in the published world when the chain allows the fallback")
	}
	if fell.Face != entity.Face("draft") || fell.ChainPosition != 1 {
		t.Errorf("POL-2 resolved to %q at position %d, want draft at 1",
			fell.Face, fell.ChainPosition)
	}

	// The assertion the labeling exists for.
	if !fell.IsFallback() {
		t.Error("a draft standing in for a missing published face MUST report " +
			"as a fallback — unlabeled, the reader sees a hit whose displayed " +
			"text need not contain the term they searched for")
	}
	if hit.IsFallback() {
		t.Error("a genuine published hit must NOT be labeled a fallback, or " +
			"the label means nothing")
	}
}

// TestSearch_ExcludedEntityIsAbsent pins that `otherwise: exclude` removes an
// entity from the world's search entirely. Absence IS the publication bit:
// an unpublished policy must not surface in a published-world search even
// though its text is indexed.
func TestSearch_ExcludedEntityIsAbsent(t *testing.T) {
	scope := store.NewWorldScope(map[string]store.TypeResolution{
		"policy": {
			Chain:    []entity.Face{"published"},
			Fallback: store.FallbackExclude,
		},
	})

	l := search.NewLinearSearch()
	face(t, l, "POL-1", "published", "secret term")
	face(t, l, "POL-2", "draft", "secret term")

	faces, err := l.Search("secret", 0, scope)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(faces) != 1 || faces[0].ID != "POL-1" {
		t.Errorf("got %v, want POL-1 only — POL-2 has no published face and "+
			"this world excludes it, so it is absent from the world entirely",
			faces)
	}
}

// TestSearch_OneHitPerEntity pins the invariant that makes `limit` count
// entities for free: a world resolves at most ONE prime per entity, so an
// entity whose faces ALL match still produces a single hit.
//
// Without it the limit would count faces, and a page of ten results could
// hold three entities.
func TestSearch_OneHitPerEntity(t *testing.T) {
	l := search.NewLinearSearch()
	face(t, l, "POL-1", "published", "duplicated term")
	face(t, l, "POL-1", "draft", "duplicated term")
	face(t, l, "POL-1", "", "duplicated term")

	faces, err := l.Search("duplicated", 0, publishedThenDraft())
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(faces) != 1 {
		t.Fatalf("three matching faces of ONE entity must yield one hit; got %d (%v)",
			len(faces), faces)
	}
	if faces[0].Face != entity.Face("published") {
		t.Errorf("the single hit must be the PRIME; got %q", faces[0].Face)
	}
}

// TestSearch_DefaultWorldIsThePreWorldsResult pins the compatibility
// contract: the zero WorldScope must return exactly what the pre-worlds
// search did — the default face of each entity, and nothing else.
//
// A non-default face must not leak in merely because it is now indexed.
// Before this ticket the stores refused to announce non-default writes at
// all, so the index could not hold them; now it does, and this is what keeps
// a faceless project's results unchanged.
func TestSearch_DefaultWorldIsThePreWorldsResult(t *testing.T) {
	l := search.NewLinearSearch()
	face(t, l, "POL-1", "", "default text")
	face(t, l, "POL-1", "draft", "draft text")
	face(t, l, "POL-2", "draft", "draft text")

	faces, err := l.Search("text", 0, store.DefaultWorld())
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(faces) != 1 || faces[0].ID != "POL-1" {
		t.Fatalf("the default world serves DEFAULT faces only; got %v", faces)
	}
	if !faces[0].Face.IsDefault() {
		t.Errorf("hit face = %q, want the default coordinate", faces[0].Face)
	}
	if faces[0].Via != search.RuleUnscoped {
		t.Errorf("via = %v, want unscoped — the default world applies no "+
			"resolution", faces[0].Via)
	}
	if faces[0].IsFallback() {
		t.Error("a default-world hit is never a fallback")
	}
	// POL-2 exists ONLY as a draft, so it is absent: the default world serves
	// default faces, and POL-2 has none.
	for _, f := range faces {
		if f.ID == "POL-2" {
			t.Error("POL-2 has no default face and must not appear in the " +
				"default world")
		}
	}
}

// TestSearch_UnscopedTypeKeepsItsDefaultFace pins resolution rule 1: a type
// the world says nothing about contributes its default state in EVERY world.
// Absence from the scope is not exclusion.
func TestSearch_UnscopedTypeKeepsItsDefaultFace(t *testing.T) {
	l := search.NewLinearSearch()
	// `ticket` is absent from the published world's scope.
	e := entity.New("TKT-1", "ticket")
	e.Content = "mixed graph term"
	if err := l.EntityPut(e); err != nil {
		t.Fatalf("EntityPut: %v", err)
	}
	face(t, l, "POL-1", "published", "mixed graph term")

	faces, err := l.Search("mixed", 0, publishedThenDraft())
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	byID := facesByID(faces)
	tkt, ok := byID["TKT-1"]
	if !ok {
		t.Fatal("an UNSCOPED type contributes its default state in every " +
			"world (rule 1) — absence from the scope is not exclusion")
	}
	if tkt.Via != search.RuleUnscoped {
		t.Errorf("TKT-1 via = %v, want unscoped", tkt.Via)
	}
	if _, ok := byID["POL-1"]; !ok {
		t.Error("control: the scoped type must still resolve")
	}
}

// TestSearch_FallbackDefaultArmIsLabelled covers the OTHER substitute shape:
// `otherwise: default`, where the chain matched nothing at all and the
// default face stood in. Both shapes must report as fallbacks, or a caller
// labeling only one leaves the other silently unlabeled.
func TestSearch_FallbackDefaultArmIsLabelled(t *testing.T) {
	scope := store.NewWorldScope(map[string]store.TypeResolution{
		"policy": {
			Chain:    []entity.Face{"published"},
			Fallback: store.FallbackDefaultState,
		},
	})

	l := search.NewLinearSearch()
	face(t, l, "POL-1", "", "substitute term")

	faces, err := l.Search("substitute", 0, scope)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(faces) != 1 {
		t.Fatalf("the default face stands in under otherwise:default; got %v", faces)
	}
	if faces[0].Via != search.RuleFallbackDefault {
		t.Errorf("via = %v, want fallback-default", faces[0].Via)
	}
	if !faces[0].IsFallback() {
		t.Error("the otherwise:default arm is a SUBSTITUTE and must be " +
			"labeled as one")
	}
}

// TestSearch_ExcludeIgnoresAnExistingDefaultFace is the sharp form of
// `otherwise: exclude`, and the one a naive fallback gets wrong.
//
// POL-2 HAS a default face carrying the term. Under `otherwise: exclude` it
// must still be absent: the world selects `published`, POL-2 has no published
// face, and exclusion means exclusion — the default face is not a consolation
// prize. Treating "a default face exists" as sufficient would publish an
// unpublished policy, which is the whole failure this world shape prevents.
//
// The earlier exclusion test seeds POL-2 with only a DRAFT face, so it passes
// even against an implementation that falls back to the default face whenever
// one exists. This one does not.
func TestSearch_ExcludeIgnoresAnExistingDefaultFace(t *testing.T) {
	scope := store.NewWorldScope(map[string]store.TypeResolution{
		"policy": {
			Chain:    []entity.Face{"published"},
			Fallback: store.FallbackExclude,
		},
	})

	l := search.NewLinearSearch()
	face(t, l, "POL-1", "published", "governed term")
	// POL-2 has a DEFAULT face — the tempting thing to fall back to — and no
	// published one.
	face(t, l, "POL-2", "", "governed term")

	faces, err := l.Search("governed", 0, scope)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(faces) != 1 || faces[0].ID != "POL-1" {
		t.Errorf("got %v, want POL-1 only — POL-2 has a default face but NO "+
			"published one, and `otherwise: exclude` means the default face is "+
			"not a substitute. Emitting it would publish an unpublished policy",
			faces)
	}
}

// TestSearch_EntityWithNoDefaultFaceIsAbsentFromAnUnscopedType pins the
// other half of resolution rule 1: an unscoped type contributes its DEFAULT
// face, so an entity that has no default face contributes NOTHING.
//
// The distinction matters because rule 1 is the arm every faceless
// project takes. An implementation that emitted rule 1 unconditionally would
// return a hit naming a face that does not exist — the caller then loads it,
// gets ErrNotFound, and the result silently vanishes from the page, which
// reads as a flaky search rather than a bug.
func TestSearch_EntityWithNoDefaultFaceIsAbsentFromAnUnscopedType(t *testing.T) {
	l := search.NewLinearSearch()
	// `ticket` is unscoped in this world. TKT-1 exists ONLY as a draft face,
	// so rule 1 has no default face to contribute.
	e := entity.New("TKT-1", "ticket")
	e.Face = "draft"
	e.Content = "orphan term"
	if err := l.EntityPut(e); err != nil {
		t.Fatalf("EntityPut: %v", err)
	}
	// A control that DOES have a default face, so the assertion below is not
	// passing against an empty result set.
	ctl := entity.New("TKT-2", "ticket")
	ctl.Content = "orphan term"
	if err := l.EntityPut(ctl); err != nil {
		t.Fatalf("EntityPut: %v", err)
	}

	faces, err := l.Search("orphan", 0, publishedThenDraft())
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	byID := facesByID(faces)
	if _, ok := byID["TKT-2"]; !ok {
		t.Fatal("control: an unscoped type WITH a default face must hit (rule 1)")
	}
	if _, ok := byID["TKT-1"]; ok {
		t.Error("TKT-1 has no DEFAULT face, and rule 1 contributes the default " +
			"face — so it contributes nothing. Emitting it would name a face " +
			"that does not exist, and the hit would vanish when loaded")
	}
}

// TestResolvePrimes_RuleOneRequiresADefaultFace tests the resolver directly,
// because no backend-level test can.
//
// LinearSearch guards against a prime naming a face it does not hold (it
// skips rather than panicking), and that guard MASKS a resolver that emits
// rule 1 for an entity with no default face — verified by mutation: removing
// the `haveDefault` condition leaves every LinearSearch test green.
//
// The bleve backend has no such guard: it compares the resolved face
// against the matched hit's, so a bogus rule-1 verdict there silently
// discards a legitimate match instead. Two backends, two different wrong
// answers from one resolver bug — so the resolver is where this is pinned.
func TestResolvePrimes_RuleOneRequiresADefaultFace(t *testing.T) {
	// `ticket` is unscoped: not a key of this scope.
	scope := store.NewWorldScope(map[string]store.TypeResolution{
		"policy": {
			Chain:    []entity.Face{"published"},
			Fallback: store.FallbackExclude,
		},
	})

	primes := search.ResolvePrimes(scope, []search.Candidate{
		// Only a draft face — no default one.
		{ID: "TKT-1", Type: "ticket", Face: "draft"},
		// Control: an unscoped type WITH a default face resolves via rule 1.
		{ID: "TKT-2", Type: "ticket", Face: ""},
	})

	if _, ok := primes["TKT-2"]; !ok {
		t.Fatal("control: an unscoped type with a default face resolves (rule 1)")
	}
	if got, ok := primes["TKT-1"]; ok {
		t.Errorf("TKT-1 resolved to %+v, want ABSENT — rule 1 contributes the "+
			"DEFAULT face, and this entity has none. Emitting it names a face "+
			"that does not exist: LinearSearch would silently skip the hit and "+
			"bleve would discard a real match", got)
	}
}

// TestResolvePrimes_ExcludeBeatsAnExistingDefaultFace is the resolver-level
// twin of TestSearch_ExcludeIgnoresAnExistingDefaultFace, pinned here too
// because `otherwise: exclude` is the arm that decides whether unpublished
// content stays unpublished. Absence IS the publication bit.
func TestResolvePrimes_ExcludeBeatsAnExistingDefaultFace(t *testing.T) {
	scope := store.NewWorldScope(map[string]store.TypeResolution{
		"policy": {
			Chain:    []entity.Face{"published"},
			Fallback: store.FallbackExclude,
		},
	})

	primes := search.ResolvePrimes(scope, []search.Candidate{
		{ID: "POL-1", Type: "policy", Face: "published"},
		// A default face exists, but the world excludes what has no
		// published face.
		{ID: "POL-2", Type: "policy", Face: ""},
	})

	if _, ok := primes["POL-1"]; !ok {
		t.Fatal("control: a published face resolves")
	}
	if got, ok := primes["POL-2"]; ok {
		t.Errorf("POL-2 resolved to %+v, want ABSENT — `otherwise: exclude` "+
			"does not fall back to the default face", got)
	}
}
