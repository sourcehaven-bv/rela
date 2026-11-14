package entitymanager

import (
	"context"
	"errors"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Deletes conditional on the version the caller read (TKT-CVT9W1K). The
// compare runs inside the delete's own transaction, so a write that lands
// between the caller's read and the delete makes the delete fail with a
// [*store.VersionConflictError] rather than delete the newer row. A data-entry
// DELETE with If-Match answers that with 412.
//
// They live on [IfMatchDeletes], not on Manager, for the reason
// CopiesForSource gives: Manager sits on its plimsoll method load line.

// IfMatchDeletes carries the conditional deletes as methods, so a consumer
// can hold them behind its own narrow interface. Each authorizes, gates and
// audits as the unconditional delete it mirrors.
type IfMatchDeletes struct{ M *Manager }

// DeleteEntity deletes the row ref names when its stored version is still
// version. The implicit face deletes the whole family, as
// [Manager.DeleteEntity] does; any other face deletes that face, as
// [Manager.DeleteEntityFace] does.
func (d IfMatchDeletes) DeleteEntity(
	ctx context.Context, ref entity.Ref, cascade bool, version store.EntityVersion,
) (*entity.DeleteResult, error) {
	pre := deleteExpect{ref: ref, version: version}
	if ref.Face.IsImplicit() {
		return deleteFamily(ctx, d.M, ref.ID, cascade, pre)
	}
	return deleteFace(ctx, d.M, ref.ID, ref.Face, cascade, pre)
}

// SoftDeleteEntity soft-deletes the family of ref's id, as
// [SoftDeleteEntity] does, when ref's stored version is still version.
func (d IfMatchDeletes) SoftDeleteEntity(
	ctx context.Context, ref entity.Ref, version store.EntityVersion,
) (*entity.DeleteResult, error) {
	return softDeleteFamily(ctx, d.M, ref.ID, deleteExpect{ref: ref, version: version})
}

// DeleteRelation deletes the relation key names, as [Manager.DeleteRelation]
// does, when its stored version is still version.
func (d IfMatchDeletes) DeleteRelation(ctx context.Context, key entity.RelationKey, version store.EntityVersion) error {
	return deleteRelation(ctx, d.M, key, version)
}

// deleteExpect is the version a conditional delete expects ref to have. The
// zero value makes the delete unconditional.
type deleteExpect struct {
	ref     entity.Ref
	version store.EntityVersion
}

// check compares ref's version, read through the transaction's view, with
// the expected one. A row that is gone is not found.
func (e deleteExpect) check(ctx context.Context, tx store.Store) error {
	if e.version == "" {
		return nil
	}
	got, err := tx.GetEntity(ctx, e.ref)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return fmt.Errorf("%w: %s", ErrEntityNotFound, e.ref.ID)
		}
		return err
	}
	if v := store.VersionOf(got); v != e.version {
		return &store.VersionConflictError{
			ID: entity.FormatStateRef(e.ref.ID, e.ref.Face), Expected: e.version, Actual: v,
		}
	}
	return nil
}

// deleteRelationIfVersion reads, compares and deletes the relation in one
// transaction. Its caller has authorized the delete and gated both ends.
// The lineage id is read before the transaction and the version and audit
// are recorded after it commits, as [Manager.ReplaceRelations] does, so a
// refused delete leaves no history behind.
//
// An empty expect skips the compare. The unconditional delete takes this
// path when a relation rule reads the edge, so the rule is checked in the
// same transaction as the delete ([checkRelationRules]).
func deleteRelationIfVersion(
	ctx context.Context, m *Manager, key entity.RelationKey, expect store.EntityVersion,
) error {
	ids, err := lineageIDs(ctx, m.deps.RelationVersionRecorder, []entity.RelationKey{key})
	if err != nil {
		return err
	}
	var rel *entity.Relation
	err = m.deps.Store.Tx(ctx, func(view store.Store) error {
		txCtx := store.ContextInTx(ctx)
		got, gErr := view.GetRelation(txCtx, key)
		if gErr != nil {
			return fmt.Errorf("delete relation: %w", gErr)
		}
		if v := store.RelationVersionOf(got); expect != "" && v != expect {
			return &store.VersionConflictError{ID: key.String(), Expected: expect, Actual: v}
		}
		if relationWritesChecked(m.deps, key) {
			if cErr := checkRelationRules(txCtx, m.deps, view, nil, []entity.RelationKey{key}); cErr != nil {
				return cErr
			}
		}
		if dErr := view.DeleteRelation(txCtx, key); dErr != nil {
			return fmt.Errorf("delete relation: %w", dErr)
		}
		rel = got
		return nil
	})
	if err != nil {
		return err
	}
	m.recordRelationVersion(ctx, store.VersionOpDelete, rel, ids[rel.Identity()], "", "", "")
	m.recordRelationAudit(ctx, audit.OpDeleteRelation, rel, "deleted")
	return nil
}
