package entitymanager

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"maps"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// checkRelationMetaWrite asks the [FieldWriteGate] whether the ctx principal
// may write the named meta keys of the relation at key, and records a
// refusal as a denied write. The gate judges the edge's source row: the
// tail's row, or every row of the source when the edge belongs to the whole
// entity (an implicit tail on a faced source), as dataentry does
// (BUG-O8QP7GX). Only a [FieldGated] handle runs it.
//
// A plain function: Manager is at its method cap.
func checkRelationMetaWrite(
	ctx context.Context, m *Manager, key entity.RelationKey, set map[string]any, unset []string,
) error {
	row, denial, err := relationMetaDenial(ctx, m, key, set, unset)
	if err != nil || denial == nil {
		return err
	}
	return recordFieldDenial(ctx, m, row, denial)
}

// relationMetaDenial is [checkRelationMetaWrite] without the audit record:
// it returns the source row the gate refused and the refusal, or a store
// error. Both are nil when the write may proceed.
func relationMetaDenial(
	ctx context.Context, m *Manager, key entity.RelationKey, set map[string]any, unset []string,
) (row *entity.Entity, denial, err error) {
	if !fieldGated(m) || (len(set) == 0 && len(unset) == 0) {
		return nil, nil, nil
	}
	family, err := lookupFamily(ctx, m.deps.Store, key.From)
	if err != nil {
		return nil, nil, err
	}
	faces := family.faces
	if !key.FromFace.IsImplicit() {
		faces = []entity.Face{key.FromFace}
	}
	for _, face := range faces {
		row, gErr := m.deps.Store.GetEntity(ctx, entity.Ref{ID: family.id, Face: face})
		if gErr != nil {
			if errors.Is(gErr, store.ErrNotFound) {
				continue // the write's own lookup reports a missing source
			}
			return nil, nil, gErr
		}
		if d := m.deps.FieldGate.CheckRelationMetaWrite(ctx, row, key.Type, set, unset); d != nil {
			return row, d, nil
		}
	}
	return nil, nil, nil
}

// renumberAllowed reports whether the renumber after an update of the edge
// at key may rewrite the order key prop of every edge q lists. Each edge's
// source must pass the relation meta gate for prop, the rule
// [checkRelationMoveMeta] applies to a move's densify: both rewrite the order
// values of other sources' edges (BUG-FQZ5Q1L). The update itself already
// passed the gate for key. A refusal is not audited: the caller asked for no
// write to those edges, and the renumber is skipped rather than refused.
//
// When allowed, judged holds every edge q listed. The renumber's transaction
// rewrites only when its own listing is a subset of judged, so an edge
// created between this check and the transaction is never rewritten
// unjudged. Nil: returned for a handle that is not field-gated, meaning no
// restriction.
func renumberAllowed(
	ctx context.Context, m *Manager, key entity.RelationKey, q store.RelationQuery, prop string,
) (judged map[entity.RelationKey]bool, allowed bool, err error) {
	if !fieldGated(m) {
		return nil, true, nil
	}
	type tail struct {
		id   string
		face entity.Face
	}
	order := map[string]any{prop: 0.0}
	seen := map[tail]bool{{id: key.From, face: key.FromFace}: true}
	judged = map[entity.RelationKey]bool{}
	for r, lErr := range m.deps.Store.ListRelations(ctx, q) {
		if lErr != nil {
			return nil, false, lErr
		}
		judged[r.Identity()] = true
		t := tail{id: r.From, face: r.FromFace}
		if seen[t] {
			continue
		}
		seen[t] = true
		_, denial, dErr := relationMetaDenial(ctx, m, r.Identity(), order, nil)
		if dErr != nil || denial != nil {
			return nil, false, dErr
		}
	}
	return judged, true, nil
}

// checkRelationMoveMeta is [checkRelationMetaWrite] for a move of the edge at
// key: the move writes the order meta key prop of every edge in readable,
// and the gate judges that key once per source tail among them. On the
// outgoing side every edge shares key's source tail; on the incoming side a
// densify rewrites edges of other sources (BUG-FQZ5Q1L), as the data-entry
// position handler judges them. The moved edge's source is judged first. The
// value is a placeholder, since the gate judges keys, not values.
//
// It runs before the move's transaction, like the ACL checks of
// [readableOrderSiblings], so a source row that changes in between is judged
// as it was read. The transaction writes only edges in readable, so no edge
// is written that the gate did not judge.
func checkRelationMoveMeta(
	ctx context.Context, m *Manager, key entity.RelationKey, readable map[entity.RelationKey]bool, prop string,
) error {
	if !fieldGated(m) {
		return nil
	}
	type tail struct {
		id   string
		face entity.Face
	}
	order := map[string]any{prop: 0.0}
	seen := map[tail]bool{}
	edges := append([]entity.RelationKey{key}, slices.SortedFunc(maps.Keys(readable), compareRelationKeys)...)
	for _, edge := range edges {
		t := tail{id: edge.From, face: edge.FromFace}
		if seen[t] {
			continue
		}
		seen[t] = true
		if err := checkRelationMetaWrite(ctx, m, edge, order, nil); err != nil {
			return err
		}
	}
	return nil
}

// compareRelationKeys orders keys by source, tail, type and target, so a
// refusal names the same edge on every run.
func compareRelationKeys(a, b entity.RelationKey) int {
	return cmp.Or(
		cmp.Compare(a.From, b.From), cmp.Compare(a.FromFace, b.FromFace),
		cmp.Compare(a.Type, b.Type), cmp.Compare(a.To, b.To),
	)
}

// checkRelationMetaUpdate is [checkRelationMetaWrite] for an update of the
// stored edge at key, and returns the options the update must write. A key
// sent with its stored value, or unset while absent, writes nothing and needs
// no grant, so it is dropped from the returned options: the write's own Tx
// reads the row again, and a dropped key cannot overwrite a change that
// landed between this check and that write. Only keys that passed the gate
// are written.
//
// A missing edge gates every key, so an edge created before the write's Tx
// still gets no unchecked key; the update's own lookup then reports a missing
// edge. Any other store error refuses the update.
func checkRelationMetaUpdate(
	ctx context.Context, m *Manager, key entity.RelationKey, opts entity.RelationOptions,
) (entity.RelationOptions, error) {
	if !fieldGated(m) || (len(opts.Properties) == 0 && len(opts.MetaUnset) == 0) {
		return opts, nil
	}
	stored, err := m.deps.Store.GetRelation(ctx, key)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			return opts, err
		}
		return opts, checkRelationMetaWrite(ctx, m, key, opts.Properties, opts.MetaUnset)
	}
	var unset []string
	for _, k := range opts.MetaUnset {
		if _, ok := stored.Properties[k]; ok {
			unset = append(unset, k)
		}
	}
	changed := ChangedRelationMeta(opts.Properties, stored.Properties)
	if err := checkRelationMetaWrite(ctx, m, key, changed, unset); err != nil {
		return opts, err
	}
	opts.Properties, opts.MetaUnset = changed, unset
	return opts, nil
}

// ChangedRelationMeta returns the keys of meta whose value differs from the
// stored edge's. A client that sends an edge's meta back unchanged writes
// nothing to those keys, so they need no meta grant. Values compare by
// their JSON form, because a decoded request and a stored edge may hold the
// same number in different Go types; a value that does not marshal counts
// as changed.
//
// Nil: meta and stored accepted (nil holds no keys); nil returned when no
// key changed.
func ChangedRelationMeta(meta, stored map[string]any) map[string]any {
	var out map[string]any
	for key, value := range meta {
		if old, ok := stored[key]; ok && sameJSON(value, old) {
			continue
		}
		if out == nil {
			out = make(map[string]any, len(meta))
		}
		out[key] = value
	}
	return out
}

func sameJSON(a, b any) bool {
	ja, errA := json.Marshal(a)
	jb, errB := json.Marshal(b)
	return errA == nil && errB == nil && bytes.Equal(ja, jb)
}
