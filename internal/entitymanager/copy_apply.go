package entitymanager

import (
	"context"
	"errors"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/automation"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// applyCopy performs the plan's writes through the transaction VIEW.
//
// Every store call here takes `view`, never the Manager's store: a write on
// the outer store from inside a Tx deadlocks on fs/mem and bypasses the
// transaction on pg. That is why the plan is fully resolved beforehand —
// this function reads nothing it did not already decide, so there is no
// temptation to reach for a handle it should not use.
func applyCopy(ctx context.Context, view store.Store, plan *copyPlan) (*CopyResult, error) {
	if plan.created {
		if err := view.CreateEntity(ctx, plan.entity); err != nil {
			return nil, fmt.Errorf("entitymanager: copy %q: create target face: %w",
				plan.name, err)
		}
	} else if err := view.UpdateEntity(ctx, plan.entity); err != nil {
		return nil, fmt.Errorf("entitymanager: copy %q: write target face: %w",
			plan.name, err)
	}

	if err := applyCopyEdges(ctx, view, plan); err != nil {
		return nil, err
	}
	return &CopyResult{
		Definition: plan.name,
		Entity:     plan.entity,
		Created:    plan.created,
	}, nil
}

// applyCopyEdges writes the copied relations, retailed to the TARGET face.
//
// Heads stay entity-level (§2.3), so a copied edge needs no rewriting on the
// far side — the world a reader stands in resolves the head. That is what
// made promote's "does it copy relations" question answerable at all.
func applyCopyEdges(ctx context.Context, view store.Store, plan *copyPlan) error {
	tail := plan.targetTail

	// `replace` removes the target face's existing edges of that type first.
	// Deliberately scoped to the TAIL: the sibling faces' edges are theirs.
	replaced := map[string]bool{}
	for _, e := range plan.edges {
		if !e.replace || replaced[e.relType] {
			continue
		}
		replaced[e.relType] = true
		// COLLECT FIRST, then delete. Mutating the store while ranging its
		// own iterator is backend-dependent: pgstore holds a pgx.Rows cursor
		// on one pooled connection while the delete takes a second, which
		// under pool pressure is a deadlock candidate.
		var doomed []*entity.Relation
		for rel, err := range view.ListRelations(ctx, store.RelationQuery{
			From: plan.targetID, Type: e.relType, FromPointer: &tail,
		}) {
			if err != nil {
				return fmt.Errorf("entitymanager: copy %q: list target edges: %w",
					plan.name, err)
			}
			doomed = append(doomed, rel)
		}
		for _, rel := range doomed {
			// Addressed BY TAIL. Dropping it here (as this did before
			// TKT-C1XUA8 PR-D) does not delete "approximately the right
			// edge": the tail is part of a relation's identity, so
			// DeleteRelation deletes the DEFAULT face's edge on the same
			// triple — a face this copy has no business touching — and
			// reports success, while the edge being replaced survives.
			derr := view.DeleteRelationState(ctx, rel.From, rel.FromPointer, rel.Type, rel.To)
			if derr != nil && !errors.Is(derr, store.ErrNotFound) {
				return fmt.Errorf("entitymanager: copy %q: replace edges: %w",
					plan.name, derr)
			}
		}
	}

	for _, e := range plan.edges {
		_, err := view.CreateRelation(ctx, plan.targetID, e.relType, e.to,
			&store.RelationData{FromPointer: tail})
		if err != nil && !errors.Is(err, store.ErrConflict) {
			// A conflict is `merge` finding the edge already present, which is
			// exactly what merge means.
			return fmt.Errorf("entitymanager: copy %q: create edge %s->%s: %w",
				plan.name, e.relType, e.to, err)
		}
	}
	return nil
}

// interpolateCopyField renders a field template against the source face.
//
// Reuses automation.Interpolate — the EXISTING {{...}} grammar, not a second
// one. It supports literal name lookup only: no expressions, no conditionals,
// no filters. `{{source.title}}` is spelled `{{new.title}}` in that grammar's
// vocabulary, so the source face binds as `new`.
func interpolateCopyField(tmpl string, src *entity.Entity) string {
	return automation.Interpolate(tmpl, automation.TemplateVars{}, src, nil)
}
