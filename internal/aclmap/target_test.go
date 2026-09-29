package aclmap_test

import (
	"context"
	"errors"
	"slices"
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
// reports that tail.
func TestCanRelation_FacedSource(t *testing.T) {
	t.Parallel()
	w := spawntWorld(t, spawntWorldPolicy)
	addFacedEntity(t, w, "TERUG-F", "terugkerend")
	ctx := context.Background()

	for _, addr := range []string{"TERUG-F", "TERUG-F@draft"} {
		res, err := w.eng.CanRelation(ctx, "SCHED", acl.VerbCreate, "spawnt", addr)
		if err != nil {
			t.Fatalf("CanRelation from %s: %v", addr, err)
		}
		if !res.Allowed || res.FromType != "terugkerend" {
			t.Errorf("CanRelation from %s = allowed %v, type %q; want allowed terugkerend",
				addr, res.Allowed, res.FromType)
		}
	}
	_, err := w.eng.CanRelation(ctx, "SCHED", acl.VerbCreate, "spawnt", "TERUG-F@review")
	if !errors.Is(err, aclmap.ErrEntityNotFound) {
		t.Errorf("CanRelation from a missing face = %v, want ErrEntityNotFound", err)
	}
}
