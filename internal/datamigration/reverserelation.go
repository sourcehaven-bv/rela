package datamigration

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/entity"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// reverseRelationStep rewrites every stored edge of a relation type so it
// points the other way (TKT-HH7PKJ), answering the relation_endpoints_swapped
// delta.
//
// # The rewrite is the store's job
//
// The step decides WHETHER a type may be reversed and the store decides HOW:
// pgstore and sqlitestore do it in one UPDATE, others fall back to a loop
// ([store.SwapRelationEndpoints] picks). That split is not only about speed.
// A relation's endpoints are its ADDRESS, so a rewrite above the store must be
// create-then-delete, and that shape destroys a self-edge, merges a
// both-directions pair, and — where the backend has history — forks every
// edge's version lineage. An in-place re-key has none of those properties: the
// row survives, so it keeps its identity and its history.
//
// # What the step still owns
//
// Everything schema-shaped, because a store must not read the metamodel:
//
//   - content-scoped types are refused (a head has no face slot, so a reversed
//     state-tailed edge is unrepresentable);
//   - types whose endpoint lists overlap are refused (an edge between two
//     entities of a shared type satisfies both orientations, so nothing can
//     tell a reversed edge from an unreversed one);
//   - symmetric types are refused (direction is meaningless, so the rewrite
//     would churn every edge to no effect).
type reverseRelationStep struct {
	Type string `yaml:"type"`
}

func (s *reverseRelationStep) Kind() string   { return "reverse_relation" }
func (s *reverseRelationStep) Target() string { return s.Type }

// resolvedSubject implements resolvingStep. Relation deltas carry a prefixed
// subject, so this must match CompareShapes' "rel:"+name exactly.
func (s *reverseRelationStep) resolvedSubject() string { return "rel:" + s.Type }

// Validate proves the reversal is expressible from the file's own projections.
//
// The endpoint checks run against the FROM shape, because that describes the
// edges as they are stored today — the ones about to be rewritten.
func (s *reverseRelationStep) Validate(from, to metamodel.ShapeProjection) error {
	if s.Type == "" {
		return errors.New("type is required")
	}
	fromRel, ok := from.Relations[s.Type]
	if !ok {
		return fmt.Errorf("relation type %q is not in the from-schema", s.Type)
	}
	toRel, ok := to.Relations[s.Type]
	if !ok {
		return fmt.Errorf("relation type %q is not in the to-schema — reversing a type the new "+
			"schema does not declare would leave every edge orphaned", s.Type)
	}
	if fromRel.Symmetric || toRel.Symmetric {
		return fmt.Errorf("relation type %q is symmetric: direction carries no meaning, so "+
			"reversing every edge would rewrite the whole type to no effect", s.Type)
	}
	if overlappingEndpoints(fromRel) {
		return fmt.Errorf("relation type %q has overlapping endpoints (from %v, to %v): an edge "+
			"between two entities of a shared type satisfies both directions, so nothing can tell "+
			"a reversed edge from one still to reverse and a re-run would flip them back",
			s.Type, fromRel.From, fromRel.To)
	}
	// The schema must actually describe the reversal the step performs.
	// Without this a hand-written step silently corrupts a type whose
	// declaration says the edges already point the right way.
	if !slices.Equal(fromRel.From, toRel.To) || !slices.Equal(fromRel.To, toRel.From) {
		return fmt.Errorf("relation type %q is not reversed between the two schemas "+
			"(from %v--%v, to %v--%v): reverse_relation rewrites stored edges to match a swap "+
			"the schema declares", s.Type, fromRel.From, fromRel.To, toRel.From, toRel.To)
	}
	return nil
}

// Run reverses the edges via the store, after the refusals the store cannot make.
//
// The scope check needs the LIVE metamodel — `scope:` is deliberately absent
// from ShapeProjection, since it does not affect whether a stored edge conforms
// — so it happens here rather than in Validate. Dry-run is the default at every
// layer above, so the refusal still reaches the operator before any write.
func (s *reverseRelationStep) Run(ctx context.Context, x *Exec) (StepResult, error) {
	res := StepResult{Kind: s.Kind(), Target: s.Target()}

	if err := s.refuseContentScoped(ctx, x); err != nil {
		return res, err
	}
	if !x.Apply {
		// A dry-run must reach the same refusals an apply would, or the
		// operator reviews a clean count for a migration that cannot run. The
		// native backends get collision detection from a unique constraint that
		// only a real write evaluates, so the check is made explicitly here and
		// costs one pass over the type.
		n, err := s.previewReversible(ctx, x)
		res.Affected = n
		return res, err
	}
	n, err := store.SwapRelationEndpoints(ctx, x.Store, s.Type)
	res.Affected = n
	return res, err
}

// refuseContentScoped rejects a type whose stored edges carry tail faces.
//
// Checked against the DATA, not the schema: `scope:` may have been changed to
// identity while state-tailed rows remain, and the whole premise of this
// subsystem is that the schema is not a reliable description of the store.
func (s *reverseRelationStep) refuseContentScoped(ctx context.Context, x *Exec) error {
	for r, err := range x.Store.ListRelations(ctx, store.RelationQuery{Type: s.Type}) {
		if err != nil {
			return err
		}
		if !r.FromFace.IsDefault() {
			return fmt.Errorf("relation %s--%s--%s is tailed at face %q: a relation's head is "+
				"entity-level by construction (there is no ToFace), so a reversed state-tailed "+
				"edge cannot be represented — reversing this type would silently drop the tail "+
				"and merge every edge that differs only by it",
				r.From, s.Type, r.To, r.FromFace)
		}
	}
	return nil
}

// previewReversible counts the edges an apply would rewrite, and refuses the
// same collisions an apply would.
//
// Self-edges are excluded from the count: reversing one is a no-op, and the
// store does not count them either, so including them here would make the
// preview disagree with the result.
//
// The collision check is duplicated from the fallback rather than shared,
// because the two answer different questions: this one must hold for EVERY
// backend on a read-only pass, while the fallback's guards the writes it is
// about to perform. A native backend has no read-only path to its own unique
// constraint, so without this a pg/sqlite dry-run would report a clean count
// for a migration that fails on apply.
func (s *reverseRelationStep) previewReversible(ctx context.Context, x *Exec) (int, error) {
	var rels []*entity.Relation
	for r, err := range x.Store.ListRelations(ctx, store.RelationQuery{Type: s.Type}) {
		if err != nil {
			return 0, err
		}
		rels = append(rels, r)
	}

	existing := make(map[string]bool, len(rels))
	for _, r := range rels {
		existing[r.Key()] = true
	}
	n := 0
	for _, r := range rels {
		if r.From == r.To {
			continue
		}
		mirror := &entity.Relation{From: r.To, Type: r.Type, To: r.From}
		if existing[mirror.Key()] {
			return 0, fmt.Errorf(
				"%w: %s--%s--%s would swap onto %s--%s--%s, which already exists — "+
					"reversing this type would merge two distinct edges",
				store.ErrConflict, r.From, r.Type, r.To, mirror.From, mirror.Type, mirror.To)
		}
		n++
	}
	return n, nil
}

// overlappingEndpoints reports whether any entity type appears on both sides.
func overlappingEndpoints(r metamodel.RelationShape) bool {
	for _, t := range r.From {
		if slices.Contains(r.To, t) {
			return true
		}
	}
	return false
}
