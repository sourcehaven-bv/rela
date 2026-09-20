package schema

import (
	"context"
	"fmt"
	"iter"
	"log/slog"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// CardinalityViolation represents a cardinality constraint violation.
type CardinalityViolation struct {
	EntityID     string
	RelationType string
	Constraint   string // "min_outgoing", "max_outgoing", "min_incoming", "max_incoming"
	Required     int
	Actual       int
}

// CardinalityReader is the read capability [CheckCardinality] requires — the
// exact two methods it calls, declared here at the CALL SITE rather than
// taking `store.Store`, for the same reason [RelationLister] is: a caller may
// pass an ACL-scoped reader, and the check then reports on the slice that
// caller can see instead of the whole graph. `store.Store` satisfies it
// structurally, so a caller holding one passes it unchanged.
//
// This lives in `schema` rather than `analysis` so that BOTH consumers can
// reach it. `internal/mcp` may not import `internal/analysis` (arch-lint;
// analysis pulls in lua, validation and validationgraph, none of which the
// MCP surface has any business holding) and reads through a gated
// GraphReader that is not a `store.Store` — so a single implementation had
// to land somewhere both sides already depend on. `analysis.CheckCardinality`
// is a thin wrapper over this (TKT-CICJSN).
type CardinalityReader interface {
	ListEntities(ctx context.Context, q store.EntityQuery) iter.Seq2[*entity.Entity, error]
	CountRelations(ctx context.Context, q store.RelationQuery) (int, error)
}

// cardinalitySpec is one direction of a relation's cardinality
// constraints: the subject population (which entity types are checked),
// the count direction, the min/max bounds with their constraint labels,
// and the relation label reported on violations (the inverse id for the
// incoming side, when declared).
//
// This is the single seam world-awareness (TKT-9KZGJO) will thread
// through: subject population, counting scope, and violation identity
// each have exactly one home — the spec, countRelationsFor, and the two
// emit passes in checkCardinalityFor (TKT-RNBLAC).
type cardinalitySpec struct {
	relName       string // metamodel relation name — the count query key
	direction     store.Direction
	subjectTypes  []string // relDef.From (outgoing) / relDef.To (incoming)
	minBound      *int     // nil or 0 disables the min check
	maxBound      *int     // nil disables the max check; 0 forbids any edge
	minConstraint string
	maxConstraint string
	relationLabel string // violation display label; inverse id on the incoming side
}

// CheckCardinality checks every cardinality constraint the metamodel
// declares, restricted to scope (nil scope checks everything).
//
// A store error fails the run loudly: the first failing count aborts
// with a wrapped error and NO violations. Reporting around a failed
// count would fabricate violations — a backend outage reads as count 0,
// which for a min bound looks exactly like missing relations
// (TKT-RNBLAC). This deliberately diverges from the under-count logging
// the surrounding analyses use: those can only miss findings, a failed
// count invents them.
func CheckCardinality(
	ctx context.Context, r CardinalityReader, meta *metamodel.Metamodel, scope map[string]bool,
) ([]CardinalityViolation, error) {
	// Non-nil even when empty: JSON callers serialize Details as [], not null.
	violations := make([]CardinalityViolation, 0)

	for relName, relDef := range meta.Relations {
		incomingLabel := relName
		if relDef.Inverse != nil && relDef.Inverse.GetID() != "" {
			incomingLabel = relDef.Inverse.GetID()
		}
		specs := [2]cardinalitySpec{
			{
				relName: relName, direction: store.DirectionOutgoing, subjectTypes: relDef.From,
				minBound: relDef.MinOutgoing, maxBound: relDef.MaxOutgoing,
				minConstraint: "min_outgoing", maxConstraint: "max_outgoing",
				relationLabel: relName,
			},
			{
				relName: relName, direction: store.DirectionIncoming, subjectTypes: relDef.To,
				minBound: relDef.MinIncoming, maxBound: relDef.MaxIncoming,
				minConstraint: "min_incoming", maxConstraint: "max_incoming",
				relationLabel: incomingLabel,
			},
		}
		for _, spec := range specs {
			v, err := checkCardinalityFor(ctx, r, meta, spec, scope)
			if err != nil {
				return nil, err
			}
			violations = append(violations, v...)
		}
	}
	return violations, nil
}

// checkCardinalityFor evaluates one direction of one relation. Each
// subject is scanned and counted once; min violations are then emitted
// before max violations (two passes over the cached counts) so the
// output order matches the historical per-constraint grouping.
func checkCardinalityFor(
	ctx context.Context, r CardinalityReader, meta *metamodel.Metamodel,
	spec cardinalitySpec, scope map[string]bool,
) ([]CardinalityViolation, error) {
	minActive := spec.minBound != nil && *spec.minBound > 0
	maxActive := spec.maxBound != nil
	if !minActive && !maxActive {
		return nil, nil
	}
	dirWord := "outgoing"
	if spec.direction == store.DirectionIncoming {
		dirWord = "incoming"
	}

	// Buffering every (subject, count) before emitting is deliberate: the
	// two emit passes below reproduce the historical min-then-max grouped
	// ordering. Collapsing this into a single count-and-emit pass would
	// interleave min and max violations and reorder the output the
	// pinning tests guard.
	type subject struct {
		id    string
		count int
	}
	var subjects []subject
	for _, subjectType := range spec.subjectTypes {
		entities := collectCardinalitySubjects(ctx, r, store.EntityQuery{Type: subjectType, AllStates: true})
		for _, e := range entities {
			if scope != nil && !scope[e.ID] {
				continue
			}
			count, err := countRelationsFor(ctx, r, meta, e, spec)
			if err != nil {
				return nil, fmt.Errorf("schema: count %s %q relations of %s: %w", dirWord, spec.relName, e.ID, err)
			}
			subjects = append(subjects, subject{id: e.ID, count: count})
		}
	}

	var violations []CardinalityViolation
	if minActive {
		for _, sub := range subjects {
			if sub.count < *spec.minBound {
				violations = append(violations, CardinalityViolation{
					EntityID:     sub.id,
					RelationType: spec.relationLabel,
					Constraint:   spec.minConstraint,
					Required:     *spec.minBound,
					Actual:       sub.count,
				})
			}
		}
	}
	if maxActive {
		for _, sub := range subjects {
			if sub.count > *spec.maxBound {
				violations = append(violations, CardinalityViolation{
					EntityID:     sub.id,
					RelationType: spec.relationLabel,
					Constraint:   spec.maxConstraint,
					Required:     *spec.maxBound,
					Actual:       sub.count,
				})
			}
		}
	}
	return violations, nil
}

// collectCardinalitySubjects materializes the subject population for one
// spec. An iterator error is logged and the partial result returned: an
// under-count can only MISS findings, where a failed relation count
// invents them — which is why that one propagates instead.
func collectCardinalitySubjects(
	ctx context.Context, r CardinalityReader, q store.EntityQuery,
) []*entity.Entity {
	out := make([]*entity.Entity, 0)
	for e, err := range r.ListEntities(ctx, q) {
		if err != nil {
			slog.Warn("schema: ListEntities iterator error; cardinality subjects may under-count",
				"type", q.Type, "error", err)
			return out
		}
		out = append(out, e)
	}
	return out
}

// countRelationsFor counts a subject's edges at the granularity the relation's
// SCOPE implies (TKT-4Y6CMV).
//
// A content-scoped edge belongs to one state on its tail side — a draft may
// implement a different control set than the published face — so its bound is
// a claim about that state and must be counted per face. Counting by bare id
// gives every state of an entity the same total, which lets one face's edge
// satisfy another face's `min_outgoing` and reports a clean graph while the
// published face genuinely has none.
//
// An identity-scoped edge belongs to the whole entity, so it is counted once
// per entity exactly as before. The tail filter is applied only on the OUTGOING
// direction, because that is the side a content scope pins; an incoming bound
// counts edges arriving at the entity regardless of which state they left.
func countRelationsFor(
	ctx context.Context, r CardinalityReader, meta *metamodel.Metamodel,
	e *entity.Entity, spec cardinalitySpec,
) (int, error) {
	q := store.RelationQuery{
		EntityID:  e.ID,
		Direction: spec.direction,
		Type:      spec.relName,
	}
	def, ok := meta.Relations[spec.relName]
	if ok && def.Scope.IsContent() && spec.direction == store.DirectionOutgoing {
		face := e.Face
		q.FromFace = &face
	}
	return r.CountRelations(ctx, q)
}
