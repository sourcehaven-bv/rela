package aclmap_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/aclmap"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// addFacedEntity stores id only at the draft face, as a faced type is stored:
// no row at the zero face (DEC-NPZICR).
func addFacedEntity(t *testing.T, w *world, id, typ string) {
	t.Helper()
	e := &entity.Entity{ID: id, Type: typ, Face: "draft", Properties: map[string]any{"title": id}}
	if err := w.store.CreateEntity(context.Background(), e); err != nil {
		t.Fatalf("seed %s@draft: %v", id, err)
	}
}

// TestCan_FacedEntity pins that the per-entity reports find a faced entity by
// its bare id, and refuse an `ID@face` address rather than answer a
// face-specific question they do not ask.
func TestCan_FacedEntity(t *testing.T) {
	t.Parallel()
	w := groundingWorld(t)
	addFacedEntity(t, w, "TKT-F", "ticket")
	ctx := context.Background()

	res, err := w.eng.Can(ctx, "PERS-ALICE", acl.VerbUpdate, "TKT-F")
	if err != nil {
		t.Fatalf("Can on a faced bare id: %v", err)
	}
	if !res.Allowed {
		t.Error("Alice (global editor) must be allowed to update the faced TKT-F")
	}

	if _, err = w.eng.Can(ctx, "PERS-ALICE", acl.VerbUpdate, "TKT-F@draft"); !errors.Is(err, aclmap.ErrFaceAddress) {
		t.Errorf("Can on TKT-F@draft = %v, want ErrFaceAddress", err)
	}
	if _, err = w.eng.Can(ctx, "PERS-ALICE", acl.VerbUpdate, "TKT-F@review"); !errors.Is(err, aclmap.ErrEntityNotFound) {
		t.Errorf("Can on a missing face = %v, want ErrEntityNotFound", err)
	}

	who, err := w.eng.WhoCan(ctx, acl.VerbUpdate, "TKT-F")
	if err != nil {
		t.Fatalf("WhoCan on a faced bare id: %v", err)
	}
	if !slices.Contains(principals(who), "PERS-ALICE") {
		t.Errorf("WhoCan update TKT-F = %v, want PERS-ALICE among them", principals(who))
	}
	if _, err := w.eng.WhoCan(ctx, acl.VerbUpdate, "TKT-F@draft"); !errors.Is(err, aclmap.ErrFaceAddress) {
		t.Errorf("WhoCan on TKT-F@draft = %v, want ErrFaceAddress", err)
	}
}

// TestCanRelation_FacedSource pins that the relation report takes a faced
// source by its bare id or by the face a content-scoped edge tails at, and
// answers as the gate does (ruling D4): a relation grant covers a named tail
// only when the principal may update that face.
func TestCanRelation_FacedSource(t *testing.T) {
	t.Parallel()
	w := spawntWorld(t, spawntWorldPolicy)
	addFacedEntity(t, w, "TERUG-F", "terugkerend")
	ctx := context.Background()

	for addr, want := range map[string]bool{"TERUG-F": true, "TERUG-F@draft": false} {
		res, err := w.eng.CanRelation(ctx, "SCHED", acl.VerbCreate, "spawnt", addr)
		if err != nil {
			t.Fatalf("CanRelation from %s: %v", addr, err)
		}
		if res.Allowed != want || res.FromType != "terugkerend" {
			t.Errorf("CanRelation from %s = allowed %v, type %q; want allowed %v, terugkerend",
				addr, res.Allowed, res.FromType, want)
		}
	}

	// No relation grant covers update, so the role grant decides, on every
	// face the family stores. A bare-type grant covers only the zero face.
	upd, err := w.eng.CanRelation(ctx, "SCHED", acl.VerbUpdate, "spawnt", "TERUG-F")
	if err != nil {
		t.Fatalf("CanRelation update from TERUG-F: %v", err)
	}
	if upd.Allowed {
		t.Error("update: [terugkerend] must not authorize a zero-tailed edge from a family stored only at draft")
	}

	faceGrant := spawntWorld(t, strings.Replace(spawntWorldPolicy,
		"update: [taak, terugkerend]", "update: [taak, terugkerend, terugkerend@draft]", 1))
	addFacedEntity(t, faceGrant, "TERUG-F", "terugkerend")
	res, err := faceGrant.eng.CanRelation(ctx, "SCHED", acl.VerbCreate, "spawnt", "TERUG-F@draft")
	if err != nil {
		t.Fatalf("CanRelation with a face grant: %v", err)
	}
	if !res.Allowed || res.RuleKind != "relation-grant" {
		t.Errorf("CanRelation with update on the draft face = allowed %v by %q; want the relation grant",
			res.Allowed, res.RuleKind)
	}

	_, err = w.eng.CanRelation(ctx, "SCHED", acl.VerbCreate, "spawnt", "TERUG-F@review")
	if !errors.Is(err, aclmap.ErrEntityNotFound) {
		t.Errorf("CanRelation from a missing face = %v, want ErrEntityNotFound", err)
	}
}
