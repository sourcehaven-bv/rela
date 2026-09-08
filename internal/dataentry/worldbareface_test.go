package dataentry

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// TKT-PI17Z6: the BARE face must be reported by its DECLARED name.
//
// A type's `bare_face:` names which declared face the zero coordinate answers
// to — it does not mint a second row (design doc §2.1). So a world chain
// compiled through metamodel.StoredFace contains "" for that face, the store
// serves a row whose Face is "", and provenance used to report exactly that:
//
//	{name: "site-nl", face: "", via: "chain", chain_position: 1}
//
// `via: chain` is CORRECT — the world's chain really did match. What was
// missing is which face it matched, and an empty string is unrenderable: every
// client falls through to printing the WORLD name, so a `site-nl` page serving
// an English guide announced "site-nl" where "en" belonged. The badge told the
// reader *a* world resolved the link and not that following it leaves Dutch.
//
// The fix is a read-back through metamodel.DeclaredFace, the inverse of the
// StoredFace mapping the chain was compiled with — never a guess, and never a
// display-layer substitution.
func bareFaceMeta(t *testing.T) *metamodel.Metamodel {
	t.Helper()
	m, err := metamodel.Parse([]byte(`
entities:
  guide:
    label: Guide
    id_prefix: GUIDE
    faces:
      en: {}
      nl: {}
    bare_face: en
    properties:
      title: {type: string}
`))
	if err != nil {
		t.Fatalf("load metamodel: %v", err)
	}
	return m
}

// siteNLScope is the multilingual chain `select: [nl, en]` AS COMPILED: `en`
// is the bare face, so it is the ZERO coordinate in the chain, at position 1.
func siteNLScope() store.WorldScope {
	return store.NewWorldScope(map[string]store.TypeResolution{
		"guide": {
			Chain:    []entity.Face{entity.Face("nl"), entity.Face("")},
			Fallback: store.FallbackExclude,
		},
	})
}

func TestWorldProvenance_BareFaceReportsDeclaredName(t *testing.T) {
	t.Parallel()

	m := bareFaceMeta(t)
	ctx := withWorld(context.Background(),
		worldHandle{name: "site-nl", scope: siteNLScope()})

	// The guide with no Dutch face: the store serves its bare (English) row.
	got := worldProvenance(ctx, m, &entity.Entity{
		ID: "GUIDE-2", Type: "guide", Face: entity.Face(""),
	})
	if got.Face != "en" {
		t.Errorf("Face = %q, want %q — the bare face is stored at the ZERO "+
			"coordinate, and reporting that coordinate raw leaves every client "+
			"printing the world name instead (TKT-PI17Z6)", got.Face, "en")
	}
	if got.Via != ruleChain {
		t.Errorf("Via = %q, want %q: the zero coordinate IS in this chain",
			got.Via, ruleChain)
	}
	// The other half of the same fix: a client cannot call this a substitute
	// without the rank, because `via` says "chain" for both candidates.
	if got.ChainPosition == nil || *got.ChainPosition != 1 {
		t.Errorf("ChainPosition = %v, want 1 — `en` is the SECOND candidate, "+
			"so this row is a within-chain stand-in for a missing `nl`",
			got.ChainPosition)
	}

	// The positive control: a non-bare face is unchanged, so the mapping is a
	// read-back and not a blanket rewrite.
	dutch := worldProvenance(ctx, m, &entity.Entity{
		ID: "GUIDE-1", Type: "guide", Face: entity.Face("nl"),
	})
	if dutch.Face != "nl" {
		t.Errorf("Face = %q, want nl — a declared coordinate passes through", dutch.Face)
	}
	if dutch.ChainPosition == nil || *dutch.ChainPosition != 0 {
		t.Errorf("ChainPosition = %v, want 0 — nl is the world's FIRST choice, "+
			"which is the claim that makes position 1 above meaningful",
			dutch.ChainPosition)
	}
}

// The view-row path must agree with the entity GET, byte for byte: the two
// label the same resolution and a client showing a list row beside a detail
// page would otherwise read two different answers for one entity.
func TestViewProvenance_BareFaceReportsDeclaredNameAndPosition(t *testing.T) {
	t.Parallel()

	m := bareFaceMeta(t)
	w := viewWorld{name: "site-nl", scope: siteNLScope()}

	got := w.provenanceFor(m, &entity.Entity{
		ID: "GUIDE-2", Type: "guide", Face: entity.Face(""),
	})
	if got.Face != "en" {
		t.Errorf("Face = %q, want en (TKT-PI17Z6)", got.Face)
	}
	// The position is the half this path was missing ENTIRELY: provenanceFor
	// used the rule-only helper, so every view row reported chain with no rank
	// and a stand-in was indistinguishable from a first-choice hit.
	if got.ChainPosition == nil || *got.ChainPosition != 1 {
		t.Errorf("ChainPosition = %v, want 1 — without it a view row cannot "+
			"say it is a substitute, which is the distinction content states "+
			"exist to make visible", got.ChainPosition)
	}

	first := w.provenanceFor(m, &entity.Entity{
		ID: "GUIDE-1", Type: "guide", Face: entity.Face("nl"),
	})
	if first.ChainPosition == nil || *first.ChainPosition != 0 {
		t.Errorf("ChainPosition = %v, want 0", first.ChainPosition)
	}
}

// A nil metamodel must degrade to the previous behavior rather than panic:
// several call sites and tests hold no metamodel, and an unknown coordinate
// has no declared name to report.
func TestWorldProvenance_NilMetamodelPassesCoordinateThrough(t *testing.T) {
	t.Parallel()

	ctx := withWorld(context.Background(),
		worldHandle{name: "site-nl", scope: siteNLScope()})
	got := worldProvenance(ctx, nil, &entity.Entity{
		ID: "GUIDE-2", Type: "guide", Face: entity.Face(""),
	})
	if got.Face != "" {
		t.Errorf("Face = %q, want \"\" — with no metamodel there is no declared "+
			"name to read back, and inventing one would be a guess", got.Face)
	}
}
