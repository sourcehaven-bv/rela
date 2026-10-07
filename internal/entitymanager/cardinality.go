package entitymanager

import (
	"context"
	"errors"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// ErrCardinalityExceeded is returned when a relation create would give an
// entity more edges of a relation than its `max_outgoing` or `max_incoming`
// allows (TKT-65LVAK). Errors carrying it are a [*CardinalityError].
var ErrCardinalityExceeded = errors.New("relation cardinality exceeded")

// CardinalityError names the relation, the bound and the entity whose edges
// are full.
type CardinalityError struct {
	Relation   string
	Constraint string // "max_outgoing" or "max_incoming"
	Limit      int
	Entity     string
}

func (e *CardinalityError) Error() string {
	return fmt.Sprintf("relation %q allows at most %d edge(s) per entity (%s), and %s has no room for another",
		e.Relation, e.Limit, e.Constraint, e.Entity)
}

func (e *CardinalityError) Unwrap() error { return ErrCardinalityExceeded }

// CheckRelationCapacity is checkRelationCapacity for a write that bypasses
// the manager, such as the dataentry soft-condition fallback.
func CheckRelationCapacity(
	ctx context.Context, st store.Store, meta *metamodel.Metamodel, key entity.RelationKey,
) error {
	return checkRelationCapacity(ctx, st, meta, key, nil)
}

// relTypeHasMax reports whether relType bounds its edges on either side, so
// creating one counts the existing edges.
func relTypeHasMax(meta *metamodel.Metamodel, relType string) bool {
	def, ok := meta.Relations[relType]
	return ok && (def.MaxOutgoing != nil || def.MaxIncoming != nil)
}

// checkRelationCapacity refuses creating key when either endpoint already
// holds the most edges of key.Type its bound allows.
//
// Only a create is refused. Data that already exceeds a bound (loaded files, a
// bound added later) stays readable and editable, and `analyze` keeps
// reporting it; it just cannot grow (TKT-65LVAK).
//
// Outgoing edges of a content-scoped relation are counted per face of the
// source, as `analyze` counts them: each face is its own set of links.
//
// st must be the Tx view the create runs in, so the count and the create see
// the same rows. Edges in leaving are not counted: a replace deletes them in
// the same Tx.
func checkRelationCapacity(
	ctx context.Context, st store.Store, meta *metamodel.Metamodel, key entity.RelationKey,
	leaving map[entity.RelationKey]bool,
) error {
	def, ok := meta.Relations[key.Type]
	if !ok {
		return nil
	}
	staying := func(r *entity.Relation) bool { return !leaving[r.Identity()] }
	if def.MaxOutgoing != nil {
		n, err := countEdges(ctx, st, store.RelationQuery{From: key.From, Type: key.Type},
			func(r *entity.Relation) bool {
				return (!def.Scope.IsContent() || r.FromFace == key.FromFace) && staying(r)
			})
		if err != nil {
			return err
		}
		if n >= *def.MaxOutgoing {
			return &CardinalityError{Relation: key.Type, Constraint: "max_outgoing",
				Limit: *def.MaxOutgoing, Entity: entity.FormatStateRef(key.From, key.FromFace)}
		}
	}
	if def.MaxIncoming != nil {
		n, err := countEdges(ctx, st, store.RelationQuery{To: key.To, Type: key.Type}, staying)
		if err != nil {
			return err
		}
		if n >= *def.MaxIncoming {
			return &CardinalityError{Relation: key.Type, Constraint: "max_incoming",
				Limit: *def.MaxIncoming, Entity: key.To}
		}
	}
	return nil
}

func countEdges(ctx context.Context, st store.Store, q store.RelationQuery, keep func(*entity.Relation) bool) (int, error) {
	n := 0
	for r, err := range st.ListRelations(ctx, q) {
		if err != nil {
			return 0, fmt.Errorf("count %s edges: %w", q.Type, err)
		}
		if keep == nil || keep(r) {
			n++
		}
	}
	return n, nil
}

// ReplaceOutgoing deletes the edges in remove and creates key when it is
// missing, in one store transaction (TKT-65LVAK). It is how a single-valued
// relation is re-pointed, such as moving a task to another status. Doing that
// as a separate delete and create could leave zero edges when the create
// fails, or two when the delete does.
//
// The caller names the edges to remove, rather than ReplaceOutgoing taking
// every edge on the tail, so it decides what the user may see and touch: an
// edge to a target the caller cannot read stays, as it does on the
// edge-by-edge path. Every edge in remove must be an outgoing key.Type edge
// of key.From on key.FromFace; one already gone is skipped. Edges in remove
// do not count against the bounds, since they go in the same Tx.
//
// The caller needs the create grant, and the delete grant when remove is not
// empty. Both are decided before the Tx from remove, which the Tx cannot
// widen. A refused or failed create leaves the edges as they were, on every
// store; see the Tx body for the order that makes this hold without rollback.
// The edge kept or created is returned.
func (m *Manager) ReplaceOutgoing(
	ctx context.Context, key entity.RelationKey, remove []entity.RelationKey, opts entity.RelationOptions,
) (*entity.Relation, error) {
	ctx = withStoreAttribution(ctx)
	if err := validTail(key.FromFace); err != nil {
		return nil, err
	}
	leaving := make(map[entity.RelationKey]bool, len(remove))
	for _, r := range remove {
		if r.From != key.From || r.FromFace != key.FromFace || r.Type != key.Type {
			return nil, fmt.Errorf("replace %s: %s --%s--> %s is not an edge of the same tail",
				key.Type, entity.FormatStateRef(r.From, r.FromFace), r.Type, r.To)
		}
		if r.To != key.To {
			leaving[r] = true
		}
	}
	rel, sourceType, err := m.prepareRelationCreate(ctx, key, opts)
	if err != nil {
		return nil, err
	}
	if len(leaving) > 0 {
		if aclErr := m.authorizeAndAudit(ctx,
			RelationDeleteRequest(m.deps.Meta, key.Type, sourceType, key.From, key.FromFace)); aclErr != nil {
			return nil, aclErr
		}
	}

	// Versions are written through the outer store, which a Tx body may not
	// use, so the pre-delete snapshots are taken first, while the rows still
	// exist (as DeleteRelation does). Should the Tx then fail, a snapshot
	// records a state that is still current, which is harmless.
	for k := range leaving {
		if r, gErr := m.deps.Store.GetRelation(ctx, k); gErr == nil {
			m.recordRelationVersion(ctx, store.VersionOpDelete, r, "", "", "")
		}
	}

	var kept *entity.Relation
	var deleted []*entity.Relation
	err = m.deps.Store.Tx(ctx, func(view store.Store) error {
		kept, deleted = nil, nil
		existing, gErr := view.GetRelation(ctx, key)
		switch {
		case gErr == nil:
			kept = existing
		case !errors.Is(gErr, store.ErrNotFound):
			return gErr
		}
		// Create before delete. Only pgstore rolls a failed Tx back; on the
		// file and memory stores a Tx only serializes. Creating first means a
		// refused or failed create changes nothing on any store, and a delete
		// failing after it leaves an extra edge that `analyze` reports, never
		// a task without a status.
		if kept == nil {
			if err := m.writeRelationCreate(ctx, view, key, rel, leaving); err != nil {
				return err
			}
		}
		for k := range leaving {
			r, gErr := view.GetRelation(ctx, k)
			if errors.Is(gErr, store.ErrNotFound) {
				continue
			}
			if gErr != nil {
				return gErr
			}
			if err := view.DeleteRelation(ctx, k); err != nil {
				return fmt.Errorf("delete relation: %w", err)
			}
			deleted = append(deleted, r)
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, store.ErrConflict) {
			return nil, fmt.Errorf("%w: %s --%s--> %s", ErrRelationAlreadyExists,
				entity.FormatStateRef(key.From, key.FromFace), key.Type, key.To)
		}
		return nil, err
	}
	for _, r := range deleted {
		m.recordRelationAudit(ctx, audit.OpDeleteRelation, r, "deleted")
	}
	if kept != nil {
		return kept, nil
	}
	m.recordRelationAudit(ctx, audit.OpCreateRelation, rel, "created")
	return rel, nil
}
