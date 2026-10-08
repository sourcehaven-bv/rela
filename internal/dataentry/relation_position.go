package dataentry

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// relationPositionWire is the `position` body of a relation PATCH: move the
// edge among its source's other edges of the type, on the outgoing order
// side. Exactly one field is set. Before and After name a sibling by its
// target id; Step moves one place up (-1) or down (1).
//
// The server computes the order value. A client that sent one would need
// the neighbors on either side, which a paged or filtered list does not
// have, and it would race any other writer.
type relationPositionWire struct {
	Before string `json:"before,omitempty"`
	After  string `json:"after,omitempty"`
	Step   int    `json:"step,omitempty"`
}

// relationPositionRequest is a position PATCH after the handler's gates:
// the path entity and the peer are readable, and key names the stored edge
// at its source's tail.
type relationPositionRequest struct {
	key       entityPkg.RelationKey
	position  relationPositionWire
	incoming  bool
	sources   []relationSource
	metaGiven bool
}

// writeRelationPosition answers a position PATCH. It runs the meta-field
// affordance gate on the order key, the same gate an explicit
// `meta: {_order_out: n}` meets, so a field the operator made read-only
// cannot be written through a move either. A sibling the principal cannot
// read and one that is not a sibling at all get the same 404 a hidden peer
// gets, so the position is no existence oracle.
//
// A package function taking the handler, not a writeHandler method: the
// handler is at its plimsoll method line.
func writeRelationPosition(w http.ResponseWriter, r *http.Request, h *writeHandler, req relationPositionRequest) {
	if req.metaGiven {
		writeV1Error(w, r, http.StatusBadRequest, "order_position_invalid",
			"position cannot be combined with meta", "")
		return
	}
	relDef, ok := h.schema().Meta.Relations[req.key.Type]
	if !ok || relDef.OutgoingOrderProperty() == "" || req.incoming {
		writeV1Error(w, r, http.StatusBadRequest, "relation_not_orderable",
			"Relation is not orderable on the outgoing side", req.key.Type)
		return
	}
	pos := entityPkg.OrderPosition{Before: req.position.Before, After: req.position.After, Step: req.position.Step}
	if pos.Before != "" && pos.After != "" {
		writeV1Error(w, r, http.StatusBadRequest, "order_position_invalid",
			"name exactly one of before, after and step", "")
		return
	}
	if ref := pos.Before + pos.After; ref != "" && !familyReadableOr404(w, r, h.visible, ref) {
		return
	}
	order := map[string]any{metamodel.OrderPropertyOut: 0.0}
	source, denial := h.affordances.relationMetaDenial(r.Context(), req.sources, req.key.Type, order, nil)
	if denial != nil {
		h.denyAfford(r.Context(), w, source, *denial)
		return
	}

	among, err := visibleSiblingTargets(r.Context(), h, req.key)
	if err != nil {
		writeGateError(w, r, err)
		return
	}
	pos.Among = among

	_, err = h.manager.UpdateRelation(r.Context(), req.key, entityPkg.RelationOptions{Position: &pos})
	switch {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case writeForbiddenIfACLDenied(w, err):
	case errors.Is(err, entitymanager.ErrOrderRefNotSibling):
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
	case errors.Is(err, entitymanager.ErrInvalidOrderPosition):
		writeV1Error(w, r, http.StatusBadRequest, "order_position_invalid", "Invalid order position", err.Error())
	case errors.Is(err, entitymanager.ErrRelationNotOrderable):
		writeV1Error(w, r, http.StatusBadRequest, "relation_not_orderable",
			"Relation is not orderable on the outgoing side", req.key.Type)
	case errors.Is(err, entitymanager.ErrRelationNotFound):
		writeV1Error(w, r, http.StatusNotFound, "relation_not_found", "Relation not found", "")
	default:
		slog.ErrorContext(r.Context(), "relation move failed", "relation", req.key.Type, "err", err)
		writeV1Error(w, r, http.StatusInternalServerError, "internal_error", "Could not move the relation", "")
	}
}

// visibleSiblingTargets lists the targets of the edges the move may
// consider: the source's edges of the type on the moved edge's own tail,
// whose endpoints the principal may read. The manager plans the move among
// these only, so neither a step nor a new value depends on an edge the
// principal cannot see. A failed read or gate is returned, never folded
// into a shorter list.
func visibleSiblingTargets(ctx context.Context, h *writeHandler, key entityPkg.RelationKey) ([]string, error) {
	var rels []*entityPkg.Relation
	for rel, err := range h.store.ListRelations(ctx, store.RelationQuery{From: key.From, Type: key.Type}) {
		if err != nil {
			return nil, err
		}
		if rel.FromFace == key.FromFace {
			rels = append(rels, rel)
		}
	}
	readable, err := h.visible.readableRelations(ctx, rels)
	if err != nil {
		return nil, err
	}
	targets := make([]string, 0, len(readable))
	for _, rel := range readable {
		targets = append(targets, rel.To)
	}
	return targets, nil
}
