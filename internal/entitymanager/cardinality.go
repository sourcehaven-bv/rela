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
// are full, and the refused edge.
type CardinalityError struct {
	Relation   string
	Constraint string // "max_outgoing" or "max_incoming"
	Limit      int
	Entity     string
	Key        entity.RelationKey // the edge whose create was refused
}

func (e *CardinalityError) Error() string {
	return fmt.Sprintf("relation %q allows at most %d edge(s) per entity (%s), and %s has no room for another",
		e.Relation, e.Limit, e.Constraint, e.Entity)
}

func (e *CardinalityError) Unwrap() error { return ErrCardinalityExceeded }

// CheckRelationCapacity is checkRelationCapacity for a write that bypasses
// the manager, such as the dataentry soft-condition fallback. Edges in
// leaving are not counted; edges in pending are counted as if stored.
func CheckRelationCapacity(
	ctx context.Context, st store.Store, meta *metamodel.Metamodel, key entity.RelationKey,
	leaving map[entity.RelationKey]bool, pending []entity.RelationKey,
) error {
	return checkRelationCapacity(ctx, st, meta, key, leaving, pending)
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
				Limit: *def.MaxOutgoing, Entity: entity.FormatStateRef(key.From, key.FromFace), Key: key}
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
				Limit: *def.MaxIncoming, Entity: key.To, Key: key}
		}
	}
	return nil
}

func countEdges(
	ctx context.Context, st store.Store, q store.RelationQuery, keep func(*entity.Relation) bool,
) (int, error) {
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

// InvalidRelationError is a relation create the metamodel's type allowlist
// refuses, while both endpoints exist. The dataentry reconciler treats it as a
// soft condition (DEC-HWZHA) and writes the edge anyway, with a warning. Its
// text stays "invalid relation: ...".
type InvalidRelationError struct {
	Key entity.RelationKey
	err error
}

func (e *InvalidRelationError) Error() string { return e.err.Error() }
func (e *InvalidRelationError) Unwrap() error { return e.err }

// RelationCreateError is a refused create of [Manager.ReplaceRelations],
// naming the edge. It wraps the refusal, so errors.Is and errors.As see
// through it, and its text is the refusal's.
type RelationCreateError struct {
	Key entity.RelationKey
	Err error
}

func (e *RelationCreateError) Error() string { return e.Err.Error() }
func (e *RelationCreateError) Unwrap() error { return e.Err }

// RelationLineageReader is an optional capability of a
// [RelationVersionRecorder]: the surrogate lineage id of a live edge, or 0
// when the edge is not live. [Manager.ReplaceRelations] reads it before its
// Tx, because a version written after the row is gone can no longer resolve
// the edge's own lineage from the key.
type RelationLineageReader interface {
	RelationRecordID(ctx context.Context, k entity.RelationKey) (int64, error)
}

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
// and every remove authorized as [Manager.DeleteRelation] does, before the Tx.
// A refused create is a [*RelationCreateError] naming the edge. A create whose
// edge already exists fails with [ErrRelationAlreadyExists]; a remove whose
// edge is already gone is skipped. When a create fails only the type
// allowlist ([*InvalidRelationError]), every other create and every remove
// has still been authorized when the error is returned.
//
// A refused replace changes no edge on any store and records no version and
// no write audit; a denied one records the denial, as every denied write
// does. See the Tx body for the order that makes this hold without rollback.
// A store fault part way through rolls back on Postgres and SQLite; on the
// file and memory stores it can leave an extra edge, never a missing one. The
// created edges are returned in the order of creates.
func (m *Manager) ReplaceRelations(
	ctx context.Context, creates []RelationCreate, removes []entity.RelationKey,
) ([]*entity.Relation, error) {
	ctx = withStoreAttribution(ctx)
	removeKeys, leaving, err := uniqueRemoves(removes)
	if err != nil {
		return nil, err
	}
	rp := relationReplace{m: m, creates: creates, removes: removeKeys, leaving: leaving}
	if prepErr := rp.prepare(ctx); prepErr != nil {
		return nil, prepErr
	}
	ids, err := lineageIDs(ctx, m.deps.RelationVersionRecorder, removeKeys)
	if err != nil {
		return nil, err
	}
	var deleted []*entity.Relation
	err = m.deps.Store.Tx(ctx, func(view store.Store) error {
		var txErr error
		deleted, txErr = rp.write(ctx, view)
		return txErr
	})
	if err != nil {
		if errors.Is(err, store.ErrConflict) {
			return nil, fmt.Errorf("%w: %w", ErrRelationAlreadyExists, err)
		}
		return nil, err
	}
	for _, r := range deleted {
		m.recordRelationVersion(ctx, store.VersionOpDelete, r, ids[r.Identity()], "", "", "")
		m.recordRelationAudit(ctx, audit.OpDeleteRelation, r, "deleted")
	}
	for _, rel := range rp.rels {
		m.recordRelationAudit(ctx, audit.OpCreateRelation, rel, "created")
	}
	return rp.rels, nil
}

// relationReplace is one ReplaceRelations call: its creates, its removes
// without repeats, and the relations prepare built for the creates.
type relationReplace struct {
	m       *Manager
	creates []RelationCreate
	removes []entity.RelationKey
	leaving map[entity.RelationKey]bool // the set of removes
	rels    []*entity.Relation          // one per create, set by prepare
}

// uniqueRemoves checks the tails of removes and drops repeats, keeping the
// order.
func uniqueRemoves(removes []entity.RelationKey) ([]entity.RelationKey, map[entity.RelationKey]bool, error) {
	leaving := make(map[entity.RelationKey]bool, len(removes))
	out := make([]entity.RelationKey, 0, len(removes))
	for _, r := range removes {
		if err := validTail(r.FromFace); err != nil {
			return nil, nil, err
		}
		if !leaving[r] {
			leaving[r] = true
			out = append(out, r)
		}
	}
	return out, leaving, nil
}

// prepare authorizes and validates the creates and authorizes the removes,
// and sets the relations to write.
//
// A create the type allowlist refuses does not stop the others from being
// authorized: a caller that writes such an edge anyway (the dataentry
// soft-condition fallback) relies on every create and remove having passed
// the ACL when that error is returned.
func (rp *relationReplace) prepare(ctx context.Context) error {
	m := rp.m
	rp.rels = make([]*entity.Relation, len(rp.creates))
	seen := make(map[entity.RelationKey]bool, len(rp.creates))
	var invalid error
	for i, c := range rp.creates {
		if rp.leaving[c.Key] || seen[c.Key] {
			return fmt.Errorf("replace relations: %s --%s--> %s is named twice",
				entity.FormatStateRef(c.Key.From, c.Key.FromFace), c.Key.Type, c.Key.To)
		}
		seen[c.Key] = true
		rel, err := prepareRelationCreate(ctx, m, c.Key, c.Opts)
		if _, soft := errors.AsType[*InvalidRelationError](err); soft {
			if invalid == nil {
				invalid = err
			}
			continue
		}
		if err != nil {
			return &RelationCreateError{Key: c.Key, Err: err}
		}
		rp.rels[i] = rel
	}
	for _, r := range rp.removes {
		// Best-effort source type, as in DeleteRelation: a lookup error leaves
		// the family zero, whose empty type matches no grant.
		source, _ := lookupFamily(ctx, m.deps.Store, r.From)
		if aclErr := m.authorizeAndAudit(ctx,
			RelationDeleteRequest(m.deps.Meta, r.Type, source.typ, r.From, r.FromFace)); aclErr != nil {
			return aclErr
		}
	}
	return invalid
}

// lineageIDs reads the lineage id of every edge in removes through rec, for
// the delete versions written after the Tx. Empty when rec cannot answer.
//
// The ids are read before the Tx, since the recorder reads through the outer
// store. That leaves a small race: an edge deleted and created again between
// this read and the Tx gets its delete version filed under the earlier
// lineage. The edges themselves are not affected.
func lineageIDs(
	ctx context.Context, rec RelationVersionRecorder, removes []entity.RelationKey,
) (map[entity.RelationKey]int64, error) {
	r, ok := rec.(RelationLineageReader)
	ids := make(map[entity.RelationKey]int64, len(removes))
	if !ok {
		return ids, nil
	}
	for _, k := range removes {
		id, err := r.RelationRecordID(ctx, k)
		if err != nil {
			return nil, fmt.Errorf("replace relations: read lineage of %s: %w", k, err)
		}
		ids[k] = id
	}
	return ids, nil
}

// write is the Tx body of ReplaceRelations, on view. It returns the edges it
// deleted, as they were.
//
// Check, then create, then delete. Only Postgres and SQLite roll a failed Tx
// back; on the file and memory stores a Tx only serializes. Every create is
// checked before any is written, so a refused replace changes nothing on any
// store. Creating before deleting means a store failure part way leaves an
// extra edge that `analyze` reports, never a task without a status.
//
// The pre-delete snapshots are read here and their versions written after
// the Tx: versions go through the outer store, which a Tx body must not use
// (the file and memory stores would deadlock), and a Tx that fails must leave
// no version behind.
func (rp *relationReplace) write(ctx context.Context, view store.Store) ([]*entity.Relation, error) {
	pending := make([]entity.RelationKey, 0, len(rp.creates))
	for _, c := range rp.creates {
		if _, gErr := view.GetRelation(ctx, c.Key); gErr == nil {
			return nil, &RelationCreateError{Key: c.Key, Err: fmt.Errorf("%w: %s --%s--> %s",
				ErrRelationAlreadyExists, entity.FormatStateRef(c.Key.From, c.Key.FromFace), c.Key.Type, c.Key.To)}
		} else if !errors.Is(gErr, store.ErrNotFound) {
			return nil, gErr
		}
		if err := checkRelationCapacity(ctx, view, rp.m.deps.Meta, c.Key, rp.leaving, pending); err != nil {
			return nil, &RelationCreateError{Key: c.Key, Err: err}
		}
		pending = append(pending, c.Key)
	}
	for i, c := range rp.creates {
		if err := writeRelationCreate(ctx, view, rp.m.deps.Meta, c.Key, rp.rels[i], rp.leaving); err != nil {
			return nil, err
		}
	}
	var deleted []*entity.Relation
	for _, k := range rp.removes {
		r, gErr := view.GetRelation(ctx, k)
		if errors.Is(gErr, store.ErrNotFound) {
			continue
		}
		if gErr != nil {
			return nil, gErr
		}
		if err := view.DeleteRelation(ctx, k); err != nil {
			return nil, fmt.Errorf("delete relation: %w", err)
		}
		deleted = append(deleted, r)
	}
	return deleted, nil
}
