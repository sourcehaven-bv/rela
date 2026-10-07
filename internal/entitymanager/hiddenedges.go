package entitymanager

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// errRelationCheck replaces the error of a failed check on a relation the
// caller cannot see. The real error is logged; its text would name the
// relation's endpoints.
var errRelationCheck = errors.New("checking its relations failed")

// edgeVisibility tells whether the caller can see an incident relation. It
// applies the read path's rule (visibility.EndpointsReadable, RR-2IK76Z),
// restated here because entitymanager may not import visibility:
//
//   - the head (To) needs some readable face;
//   - the tail (From) of a content-scoped edge needs its own face readable;
//     a tail with no face is judged like the head.
//
// A nil gate sees every relation.
type edgeVisibility struct {
	gate  faceReadGate
	st    store.EntityLister
	faces map[string]map[entity.Face]bool // readable faces per endpoint id
}

func newEdgeVisibility(gate faceReadGate, st store.EntityLister) *edgeVisibility {
	return &edgeVisibility{gate: gate, st: st, faces: make(map[string]map[entity.Face]bool)}
}

// seed records which of rows the caller can read. It is for rows a store
// read does not return, such as the faces of a soft-deleted entity being
// restored.
func (v *edgeVisibility) seed(ctx context.Context, rows []*entity.Entity) error {
	if v.gate == nil {
		return nil
	}
	readable, err := readableRows(ctx, v.gate, rows)
	if err != nil {
		return err
	}
	for _, e := range rows {
		if v.faces[e.ID] == nil {
			v.faces[e.ID] = make(map[entity.Face]bool)
		}
	}
	for _, e := range readable {
		v.faces[e.ID][e.Face] = true
	}
	return nil
}

// load gates every stored face of the endpoints of rels it does not know
// yet. The headers come from one store query; each row is then put to the
// gate.
func (v *edgeVisibility) load(ctx context.Context, rels []*entity.Relation) error {
	if v.gate == nil {
		return nil
	}
	loaded := make(map[string]map[entity.Face]bool)
	var ids []string
	for _, rel := range rels {
		if rel == nil {
			continue
		}
		for _, id := range [2]string{rel.From, rel.To} {
			if _, known := v.faces[id]; known {
				continue
			}
			if _, queued := loaded[id]; !queued {
				loaded[id] = make(map[entity.Face]bool)
				ids = append(ids, id)
			}
		}
	}
	if len(ids) == 0 {
		return nil
	}
	for h, err := range store.ListEntityHeaders(ctx, v.st, store.EntityQuery{IDs: ids, Faces: store.AllFaces()}) {
		if err != nil {
			return err
		}
		if _, asked := loaded[h.ID]; !asked {
			continue
		}
		ok, gErr := v.gate.PermitsReadFace(ctx, h.Type, h.ID, h.Face)
		if gErr != nil {
			return gErr
		}
		if ok {
			loaded[h.ID][h.Face] = true
		}
	}
	maps.Copy(v.faces, loaded)
	return nil
}

// visible reports whether the caller can see rel. Call load first: an
// endpoint it has not loaded counts as unreadable.
func (v *edgeVisibility) visible(rel *entity.Relation) bool {
	if v.gate == nil {
		return true
	}
	if len(v.faces[rel.To]) == 0 {
		return false
	}
	if rel.FromFace.IsImplicit() {
		return len(v.faces[rel.From]) > 0
	}
	return v.faces[rel.From][rel.FromFace]
}

// hiddenEdgeFailure is the error of a delete blocked by a relation the
// caller cannot see. A denial reveals that the delete was refused, the one
// bit docs/acl-security.md accepts. The decision names no rule from the real
// check, and the edge's type and endpoints are withheld. Any other failure is
// logged and reported as [errRelationCheck], which is not a 403.
func hiddenEdgeFailure(id string, err error) error {
	if errors.Is(err, acl.ErrForbidden) {
		return fmt.Errorf("cannot delete %s: %w", id, &acl.ForbiddenError{Decision: acl.Decision{
			RuleKind: "relation",
			RuleID:   "-",
			Reason:   "a relation blocks this operation",
		}})
	}
	slog.Error("entitymanager: relation check failed on a delete", "id", id, "error", err)
	return fmt.Errorf("cannot delete %s: %w", id, errRelationCheck)
}

// readableRelationCount counts the incident relations of id the caller can
// see, each relation once.
func readableRelationCount(ctx context.Context, gate faceReadGate, st store.Store, id string) (int, error) {
	rels, err := collectRelations(ctx, st, store.RelationQuery{EntityID: id, Direction: store.DirectionBoth})
	if err != nil {
		return 0, err
	}
	vis := newEdgeVisibility(gate, st)
	if err := vis.load(ctx, rels); err != nil {
		return 0, err
	}
	n := 0
	for _, rel := range rels {
		if vis.visible(rel) {
			n++
		}
	}
	return n, nil
}
