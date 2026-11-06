package entitymanager

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"

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
	if !fieldGated(m) || (len(set) == 0 && len(unset) == 0) {
		return nil
	}
	family, err := lookupFamily(ctx, m.deps.Store, key.From)
	if err != nil {
		return err
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
			return gErr
		}
		denial := m.deps.FieldGate.CheckRelationMetaWrite(ctx, row, key.Type, set, unset)
		if denial != nil {
			return recordFieldDenial(ctx, m, row, denial)
		}
	}
	return nil
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
