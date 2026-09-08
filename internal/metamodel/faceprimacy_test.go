package metamodel

import (
	"strings"
	"testing"
)

// The face-switcher asks "which world serves this face?" and the answer is
// INFERRED from the compiled chains. These tests pin the three ways that
// inference can go wrong, all of which are load errors rather than a silent
// pick (TKT-MFVH03).

func parseWorlds(t *testing.T, doc string) (*Metamodel, error) {
	t.Helper()
	return Parse([]byte(doc))
}

// twoTypeSchema declares two worlds that BOTH lead with `nl` for `guide`,
// which is the ambiguity `primary_for:` exists to resolve. The `worlds:` block
// is supplied by each test.
const primacyPrefix = `
version: "1"
entities:
  guide:
    label: Guide
    id_prefix: GUIDE
    bare_face: en
    faces:
      en: {}
      nl: {}
    properties:
      title: {type: string}
worlds:
`

func TestFacePrimacy_UndeclaredTieIsALoadError(t *testing.T) {
	_, err := parseWorlds(t, primacyPrefix+`
  site-nl:
    select: [nl, en]
    otherwise: default
  editorial-nl:
    select: [nl]
    otherwise: default
`)
	if err == nil {
		t.Fatal("two worlds heading `nl` identically with no `primary_for:` must fail the load — " +
			"otherwise the face-switcher picks one by map iteration order")
	}
	for _, want := range []string{"guide", "site-nl", "editorial-nl", "nl", "primary_for"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should name %q so the operator can act on it; got: %v", want, err)
		}
	}
}

func TestFacePrimacy_DeclaringOneResolvesTheTie(t *testing.T) {
	m, err := parseWorlds(t, primacyPrefix+`
  site-nl:
    select: [nl, en]
    otherwise: default
    primary_for: nl
  editorial-nl:
    select: [nl]
    otherwise: default
`)
	if err != nil {
		t.Fatalf("one claimant resolves the ambiguity, so this must load: %v", err)
	}
	if got := m.Worlds["site-nl"].PrimaryFor; len(got) != 1 || got[0] != "nl" {
		t.Fatalf("PrimaryFor did not survive the load: %v", got)
	}
}

func TestFacePrimacy_TwoClaimantsIsALoadError(t *testing.T) {
	_, err := parseWorlds(t, primacyPrefix+`
  site-nl:
    select: [nl, en]
    otherwise: default
    primary_for: nl
  editorial-nl:
    select: [nl]
    otherwise: default
    primary_for: nl
`)
	if err == nil {
		t.Fatal("a face has ONE canonical home; two claimants must fail rather than " +
			"reintroduce the arbitrary pick this key removes")
	}
}

// A claim may only CONFIRM the chains. Naming a face the world merely falls
// back to would produce a control navigating somewhere the face is not
// primary — the affordance that lies.
func TestFacePrimacy_ClaimingAFaceTheWorldDoesNotHead(t *testing.T) {
	_, err := parseWorlds(t, primacyPrefix+`
  site-nl:
    select: [nl, en]
    otherwise: default
    primary_for: en
`)
	if err == nil {
		t.Fatal("`en` is site-nl's FALLBACK, not its head — claiming it must fail")
	}
	if !strings.Contains(err.Error(), "does not head") {
		t.Errorf("error should say the world does not head the face; got: %v", err)
	}
}

// Sharing a chain head is NOT a tie when the worlds resolve absence
// differently. This pair is the prototype's own shape — a published world where
// absence means "not published" and a lenient sibling that substitutes instead
// — and rejecting it would fail a working schema for a question the operator
// has already answered with `otherwise:`.
//
// This is the case that caught an over-broad first version of the rule
// (internal/worlds TestCompile_ChainsAndFallback), so it is pinned here.
func TestFacePrimacy_SameHeadDifferentOtherwiseIsNotATie(t *testing.T) {
	if _, err := parseWorlds(t, primacyPrefix+`
  published:
    select: [nl]
    otherwise: exclude
  lenient:
    select: [nl]
    otherwise: default
`); err != nil {
		t.Fatalf("two worlds may lead the same face while answering different "+
			"questions about entities lacking it: %v", err)
	}
}

// One world leading a face needs no declaration at all: inference is the
// default and the common case stays zero-config.
func TestFacePrimacy_SingleHeadNeedsNoDeclaration(t *testing.T) {
	if _, err := parseWorlds(t, primacyPrefix+`
  site-nl:
    select: [nl, en]
    otherwise: default
`); err != nil {
		t.Fatalf("an unambiguous schema must load without `primary_for:`: %v", err)
	}
}

// Headship is per (type, face) because `overrides:` makes it so. Two worlds
// leading DIFFERENT faces for the same type are not in conflict, and a world
// whose override moves it off a face stops competing for it.
func TestFacePrimacy_PerTypeHeadshipIsNotATie(t *testing.T) {
	doc := `
version: "1"
entities:
  guide:
    label: Guide
    id_prefix: GUIDE
    bare_face: en
    faces: {en: {}, nl: {}}
    properties: {title: {type: string}}
  policy:
    label: Policy
    id_prefix: POL
    bare_face: en
    faces: {en: {}, nl: {}}
    properties: {title: {type: string}}
worlds:
  # Leads nl for guide, en for policy.
  a:
    select: [en]
    overrides:
      guide: [nl]
    otherwise: default
  # Leads nl for policy and en for guide, so no pair is led by both worlds.
  b:
    select: [nl]
    overrides:
      guide: [en]
    otherwise: default
`
	if _, err := parseWorlds(t, doc); err != nil {
		t.Fatalf("no (type, face) pair is led by both worlds, so this must load: %v", err)
	}
}

// The mirror of the case above: the SAME two worlds become ambiguous as soon
// as their chains collide on one pair. This is what proves the test above is
// passing because headship is per-type and not because the check is inert.
func TestFacePrimacy_CollisionOnOnePairStillFails(t *testing.T) {
	doc := `
version: "1"
entities:
  guide:
    label: Guide
    id_prefix: GUIDE
    bare_face: en
    faces: {en: {}, nl: {}}
    properties: {title: {type: string}}
  policy:
    label: Policy
    id_prefix: POL
    bare_face: en
    faces: {en: {}, nl: {}}
    properties: {title: {type: string}}
worlds:
  a:
    select: [en]
    overrides:
      guide: [nl]
    otherwise: default
  # Same as world a for guide — both now lead (guide, nl).
  b:
    select: [nl]
    overrides:
      guide: [nl]
    otherwise: default
`
	err := mustFail(t, doc)
	if !strings.Contains(err.Error(), "guide") {
		t.Errorf("the error should name the CONTESTED type, not another one; got: %v", err)
	}
	if strings.Contains(err.Error(), "policy") {
		t.Errorf("policy is led by `b` alone and must not be reported; got: %v", err)
	}
}

func mustFail(t *testing.T, doc string) error {
	t.Helper()
	_, err := parseWorlds(t, doc)
	if err == nil {
		t.Fatal("expected a load error, got none")
	}
	return err
}
