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
	return fmt.Sprintf("relation %q allows at most %d edge(s) per entity (%s); %s already has %d",
		e.Relation, e.Limit, e.Constraint, e.Entity, e.Limit)
}

func (e *CardinalityError) Unwrap() error { return ErrCardinalityExceeded }

// relTypeHasMax reports whether relType bounds its edges on either side, so
// creating one counts the existing edges.
// CheckRelationCapacity is checkRelationCapacity for a write that bypasses
// the manager, such as the dataentry soft-condition fallback.
func CheckRelationCapacity(
	ctx context.Context, st store.Store, meta *metamodel.Metamodel, key entity.RelationKey,
) error {
	return checkRelationCapacity(ctx, st, meta, key, true)
}

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
// the same rows.
//
// checkOutgoing false skips the source's bound, for a replace that removes the
// source's other edges in the same Tx.
func checkRelationCapacity(
	ctx context.Context, st store.Store, meta *metamodel.Metamodel, key entity.RelationKey,
	checkOutgoing bool,
) error {
	def, ok := meta.Relations[key.Type]
	if !ok {
		return nil
	}
	if checkOutgoing && def.MaxOutgoing != nil {
		n, err := countEdges(ctx, st, store.RelationQuery{From: key.From, Type: key.Type},
			func(r *entity.Relation) bool { return !def.Scope.IsContent() || r.FromFace == key.FromFace })
		if err != nil {
			return err
		}
		if n >= *def.MaxOutgoing {
			return &CardinalityError{Relation: key.Type, Constraint: "max_outgoing",
				Limit: *def.MaxOutgoing, Entity: entity.FormatStateRef(key.From, key.FromFace)}
		}
	}
	if def.MaxIncoming != nil {
		n, err := countEdges(ctx, st, store.RelationQuery{To: key.To, Type: key.Type}, nil)
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

// ReplaceOutgoing makes key the only outgoing edge of key.Type on key.From's
// tail (key.FromFace): it deletes every other such edge and creates key when
// it is missing, in one store transaction (TKT-65LVAK). A refused or failed
// create leaves the edges as they were, on every store; see the Tx body for
// the order that makes this hold without rollback.
//
// It is how a single-valued relation is re-pointed, such as moving a task to
// another status. Doing that as a delete and a create could leave zero edges
// when the create fails, or two when the delete does.
//
// The caller needs both grants: delete on the relation (when there is an edge
// to remove) and create. The edge kept or created is returned.
func (m *Manager) ReplaceOutgoing(
	ctx context.Context, key entity.RelationKey, opts entity.RelationOptions,
) (*entity.Relation, error) {
	ctx = withStoreAttribution(ctx)
	if err := validTail(key.FromFace); err != nil {
		return nil, err
	}
	rel, sourceType, err := m.prepareRelationCreate(ctx, key, opts)
	if err != nil {
		return nil, err
	}

	sameTail := func(r *entity.Relation) bool { return r.FromFace == key.FromFace }
	before, err := m.outgoingOnTail(ctx, m.deps.Store, key, sameTail)
	if err != nil {
		return nil, err
	}
	var doomed []*entity.Relation
	for _, r := range before {
		if r.To != key.To {
			doomed = append(doomed, r)
		}
	}
	if len(doomed) > 0 {
		if aclErr := m.authorizeAndAudit(ctx,
			RelationDeleteRequest(m.deps.Meta, key.Type, sourceType, key.From, key.FromFace)); aclErr != nil {
			return nil, aclErr
		}
	}
	// Versions are written through the outer store, which a Tx body may not
	// use, so the pre-delete snapshots are taken first, while the rows still
	// exist (as DeleteRelation does). Should the Tx then fail, the snapshot
	// records a state that is still current, which is harmless.
	for _, r := range doomed {
		m.recordRelationVersion(ctx, store.VersionOpDelete, r, "", "", "")
	}

	var kept *entity.Relation
	var deleted []*entity.Relation
	err = m.deps.Store.Tx(ctx, func(view store.Store) error {
		kept, deleted = nil, nil
		current, err := m.outgoingOnTail(ctx, view, key, sameTail)
		if err != nil {
			return err
		}
		for _, r := range current {
			if r.To == key.To {
				kept = r
			}
		}
		// Create before delete. Only pgstore rolls a failed Tx back; on the
		// file and memory stores a Tx only serializes. Creating first means a
		// refused or failed create changes nothing on any store, and a delete
		// failing after it leaves an extra edge that `analyze` reports, never
		// a task without a status.
		if kept == nil {
			if err := m.writeRelationCreate(ctx, view, key, rel, false); err != nil {
				return err
			}
		}
		for _, r := range current {
			if r.To == key.To {
				continue
			}
			if err := view.DeleteRelation(ctx, r.Identity()); err != nil {
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

func (m *Manager) outgoingOnTail(
	ctx context.Context, st store.Store, key entity.RelationKey, keep func(*entity.Relation) bool,
) ([]*entity.Relation, error) {
	var out []*entity.Relation
	for r, err := range st.ListRelations(ctx, store.RelationQuery{From: key.From, Type: key.Type}) {
		if err != nil {
			return nil, fmt.Errorf("list %s edges: %w", key.Type, err)
		}
		if keep(r) {
			out = append(out, r)
		}
	}
	return out, nil
}
