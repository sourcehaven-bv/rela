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
// the manager, such as the dataentry soft-condition fallback. Edges in
// leaving are not counted; nil counts every edge.
func CheckRelationCapacity(
	ctx context.Context, st store.Store, meta *metamodel.Metamodel, key entity.RelationKey,
	leaving map[entity.RelationKey]bool,
) error {
	return checkRelationCapacity(ctx, st, meta, key, leaving, nil)
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
// the same Tx. Edges in pending are counted although not stored yet: a
// replace creates them in the same Tx, before key.
func checkRelationCapacity(
	ctx context.Context, st store.Store, meta *metamodel.Metamodel, key entity.RelationKey,
	leaving map[entity.RelationKey]bool, pending []entity.RelationKey,
) error {
	def, ok := meta.Relations[key.Type]
	if !ok {
		return nil
	}
	staying := func(r *entity.Relation) bool { return !leaving[r.Identity()] }
	sameTail := func(from string, face entity.Face) bool {
		return from == key.From && (!def.Scope.IsContent() || face == key.FromFace)
	}
	if def.MaxOutgoing != nil {
		n, err := countEdges(ctx, st, store.RelationQuery{From: key.From, Type: key.Type},
			func(r *entity.Relation) bool { return sameTail(r.From, r.FromFace) && staying(r) })
		if err != nil {
			return err
		}
		for _, p := range pending {
			if p.Type == key.Type && sameTail(p.From, p.FromFace) {
				n++
			}
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
		for _, p := range pending {
			if p.Type == key.Type && p.To == key.To {
				n++
			}
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

// invalidRelationError marks a relation the metamodel's type allowlist
// refuses (both endpoints exist). Its text stays "invalid relation: ...",
// which callers match on.
type invalidRelationError struct{ err error }

func (e *invalidRelationError) Error() string { return e.err.Error() }
func (e *invalidRelationError) Unwrap() error { return e.err }

// RelationCreate is one edge [Manager.ReplaceRelations] creates.
type RelationCreate struct {
	Key  entity.RelationKey
	Opts entity.RelationOptions
}

// ReplaceRelations creates the edges in creates and deletes the edges in
// removes, in one store transaction (TKT-65LVAK). It is how a bounded
// relation is re-pointed, such as moving a task to another status. Doing that
// as a separate delete and create could leave zero edges when the create
// fails, or exceed the bound when the create runs first.
//
// The edges may be of any relation types and touch any entities. The caller
// names the edges to remove, rather than the manager taking every edge on a
// side, so it decides what the user may see and touch: an edge the caller
// cannot read stays, as it does on the edge-by-edge path. Edges in removes do
// not count against the bounds of the creates. Earlier creates do count, so
// two creates on a `max_outgoing: 2` side fit and a third does not.
//
// Every create is authorized and validated as [Manager.CreateRelation] does,
// and every remove as [Manager.DeleteRelation] does, before the Tx. A create
// whose edge already exists fails with [ErrRelationAlreadyExists]; a remove
// whose edge is already gone is skipped.
//
// A refused replace (by a bound, an existing edge, the ACL or validation)
// changes no edge on any store and records no version and no audit; see the
// Tx body for the order that makes this hold without rollback. A store fault
// part way through rolls back on Postgres; on the file and memory stores it
// can leave an extra edge, never a missing one. The created edges are
// returned in the order of creates.
func (m *Manager) ReplaceRelations(
	ctx context.Context, creates []RelationCreate, removes []entity.RelationKey,
) ([]*entity.Relation, error) {
	ctx = withStoreAttribution(ctx)
	leaving := make(map[entity.RelationKey]bool, len(removes))
	var removeKeys []entity.RelationKey
	for _, r := range removes {
		if err := validTail(r.FromFace); err != nil {
			return nil, err
		}
		if !leaving[r] {
			leaving[r] = true
			removeKeys = append(removeKeys, r)
		}
	}
	rels := make([]*entity.Relation, len(creates))
	seen := make(map[entity.RelationKey]bool, len(creates))
	// A create the metamodel's type allowlist refuses does not stop the
	// others from being authorized: a caller that writes such an edge anyway
	// (the dataentry soft-condition fallback) relies on every create and
	// remove having passed the ACL when this error is returned.
	var invalid error
	for i, c := range creates {
		if leaving[c.Key] || seen[c.Key] {
			return nil, fmt.Errorf("replace relations: %s --%s--> %s is named twice",
				entity.FormatStateRef(c.Key.From, c.Key.FromFace), c.Key.Type, c.Key.To)
		}
		seen[c.Key] = true
		rel, _, err := m.prepareRelationCreate(ctx, c.Key, c.Opts)
		var ie *invalidRelationError
		if errors.As(err, &ie) {
			if invalid == nil {
				invalid = err
			}
			continue
		}
		if err != nil {
			return nil, err
		}
		rels[i] = rel
	}
	for _, r := range removeKeys {
		// Best-effort source type, as in DeleteRelation: a lookup error leaves
		// the family zero, whose empty type matches no grant.
		source, _ := lookupFamily(ctx, m.deps.Store, r.From)
		if aclErr := m.authorizeAndAudit(ctx,
			RelationDeleteRequest(m.deps.Meta, r.Type, source.typ, r.From, r.FromFace)); aclErr != nil {
			return nil, aclErr
		}
	}
	if invalid != nil {
		return nil, invalid
	}

	type removed struct {
		rel      *entity.Relation
		recordID int64
	}
	var deleted []removed
	err := m.deps.Store.Tx(ctx, func(view store.Store) error {
		deleted = nil
		// Check, then create, then delete. Only pgstore rolls a failed Tx
		// back; on the file and memory stores a Tx only serializes. Every
		// create is checked before any is written, so a refused replace
		// changes nothing on any store. Creating before deleting means a
		// store failure part way leaves an extra edge that `analyze`
		// reports, never a task without a status.
		pending := make([]entity.RelationKey, 0, len(creates))
		for _, c := range creates {
			if _, gErr := view.GetRelation(ctx, c.Key); gErr == nil {
				return fmt.Errorf("%w: %s --%s--> %s", ErrRelationAlreadyExists,
					entity.FormatStateRef(c.Key.From, c.Key.FromFace), c.Key.Type, c.Key.To)
			} else if !errors.Is(gErr, store.ErrNotFound) {
				return gErr
			}
			if err := checkRelationCapacity(ctx, view, m.deps.Meta, c.Key, leaving, pending); err != nil {
				return err
			}
			pending = append(pending, c.Key)
		}
		for i, c := range creates {
			if err := m.writeRelationCreate(ctx, view, c.Key, rels[i], leaving); err != nil {
				return err
			}
		}
		// The pre-delete snapshots are read here, in the Tx, and their
		// versions written after it: versions go through the outer store,
		// which a Tx body must not use (the file and memory stores would
		// deadlock), and a Tx that fails must leave no version behind. The
		// lineage id is read now, while the row exists, since a version
		// written later can no longer resolve it from the key.
		ids, _ := view.(store.RelationRecordIDReader)
		for _, k := range removeKeys {
			r, gErr := view.GetRelation(ctx, k)
			if errors.Is(gErr, store.ErrNotFound) {
				continue
			}
			if gErr != nil {
				return gErr
			}
			var id int64
			if ids != nil {
				if id, gErr = ids.RelationRecordID(ctx, k); gErr != nil && !errors.Is(gErr, store.ErrNotFound) {
					return gErr
				}
			}
			if err := view.DeleteRelation(ctx, k); err != nil {
				return fmt.Errorf("delete relation: %w", err)
			}
			deleted = append(deleted, removed{rel: r, recordID: id})
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, store.ErrConflict) {
			return nil, fmt.Errorf("%w: %w", ErrRelationAlreadyExists, err)
		}
		return nil, err
	}
	for _, d := range deleted {
		m.recordRelationVersionAt(ctx, store.VersionOpDelete, d.rel, d.recordID, "", "", "")
		m.recordRelationAudit(ctx, audit.OpDeleteRelation, d.rel, "deleted")
	}
	for _, rel := range rels {
		m.recordRelationAudit(ctx, audit.OpCreateRelation, rel, "created")
	}
	return rels, nil
}
