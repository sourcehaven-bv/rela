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

// facedTerugkerend is a schema in which terugkerend declares draft and
// published.
func facedTerugkerend(entityType string) []entity.Face {
	if entityType == "terugkerend" {
		return []entity.Face{"draft", "published"}
	}
	return nil
}

// withFaces rebuilds w's engine over the schema faces describes.
func withFaces(t *testing.T, w *world, faces aclmap.DeclaredFaces) *world {
	t.Helper()
	eng, err := aclmap.New(w.store, w.decl, faces)
	if err != nil {
		t.Fatalf("aclmap.New: %v", err)
	}
	return &world{store: w.store, decl: w.decl, eng: eng}
}

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
	w := withFaces(t, spawntWorld(t, spawntWorldPolicy), facedTerugkerend)
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
	// face the TYPE declares. A bare-type grant covers only the zero face,
	// and a grant on the one stored face is not enough either: the answer
	// must not depend on which faces this entity stores.
	for _, tc := range []struct {
		name, grant string
		want        bool
	}{
		{"bare type", "update: [taak, terugkerend]", false},
		{"stored face only", "update: [taak, terugkerend@draft]", false},
		{"every declared face", "update: [taak, terugkerend@draft, terugkerend@published]", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ww := withFaces(t, spawntWorld(t, strings.Replace(spawntWorldPolicy,
				"update: [taak, terugkerend]", tc.grant, 1)), facedTerugkerend)
			addFacedEntity(t, ww, "TERUG-F", "terugkerend")
			upd, err := ww.eng.CanRelation(t.Context(), "SCHED", acl.VerbUpdate, "spawnt", "TERUG-F")
			if err != nil {
				t.Fatalf("CanRelation update from TERUG-F: %v", err)
			}
			if upd.Allowed != tc.want {
				t.Errorf("%s: zero-tailed update from TERUG-F allowed = %v, want %v (%s)",
					tc.grant, upd.Allowed, tc.want, upd.Reason)
			}
		})
	}

	faceGrant := withFaces(t, spawntWorld(t, strings.Replace(spawntWorldPolicy,
		"update: [taak, terugkerend]", "update: [taak, terugkerend, terugkerend@draft]", 1)), facedTerugkerend)
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
