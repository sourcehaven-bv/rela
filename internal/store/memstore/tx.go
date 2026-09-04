package memstore

import (
	"context"
	"io"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Tx and the txMu write serialization (store.Transactor, DEC-8UIL0).
//
// Same shape and same reduced guarantees as fsstore (see fsstore/tx.go):
// Tx holds txMu across the callback for mutual exclusion with ordinary
// writers; no rollback; events/observer notifications emitted inline per
// write. fn receives a txStore view whose write methods call the
// unexported cores, skipping txMu — writes on the outer store from
// inside fn deadlock.

// Tx implements store.Transactor with mutual exclusion only (no
// rollback; events emitted inline).
func (m *MemStore) Tx(_ context.Context, fn func(store.Store) error) error {
	m.txMu.Lock()
	defer m.txMu.Unlock()
	return fn(txStore{m})
}

// lockTx takes the Tx serialization lock and returns its release func,
// letting the exported write wrappers stay one-liners.
func (m *MemStore) lockTx() func() {
	m.txMu.Lock()
	return m.txMu.Unlock
}

// --- exported write API: serialize with any open Tx, then delegate ---

// CreateEntity implements store.EntityWriter.
// Returns store.ErrConflict if an entity with the same ID exists.
func (m *MemStore) CreateEntity(ctx context.Context, e *entity.Entity) error {
	defer m.lockTx()()
	return m.createEntity(ctx, e)
}

// UpdateEntity implements store.EntityWriter.
// Returns store.ErrNotFound if the entity does not exist.
func (m *MemStore) UpdateEntity(ctx context.Context, e *entity.Entity) error {
	defer m.lockTx()()
	return m.updateEntity(ctx, e)
}

// DeleteEntity implements store.EntityWriter.
// Returns store.ErrNotFound if the entity does not exist.
func (m *MemStore) DeleteEntity(ctx context.Context, id string, cascade bool) (*store.DeleteResult, error) {
	defer m.lockTx()()
	return m.deleteEntity(ctx, id, cascade)
}

// DeleteEntityState implements store.EntityWriter.
// Returns store.ErrNotFound if that face does not exist.
func (m *MemStore) DeleteEntityState(
	ctx context.Context, id string, p entity.Face,
) (*store.DeleteResult, error) {
	defer m.lockTx()()
	return m.deleteEntityState(ctx, id, p)
}

// RenameEntity implements store.EntityWriter.
// Returns store.ErrNotFound if oldID is absent, store.ErrConflict if
// newID exists.
func (m *MemStore) RenameEntity(ctx context.Context, oldID, newID string) (*store.RenameResult, error) {
	defer m.lockTx()()
	return m.renameEntity(ctx, oldID, newID)
}

// CreateRelation implements store.RelationWriter.
func (m *MemStore) CreateRelation(
	ctx context.Context, from, relType, to string, data *store.RelationData,
) (*entity.Relation, error) {
	defer m.lockTx()()
	return m.createRelation(ctx, from, relType, to, data)
}

// UpdateRelation implements store.RelationWriter.
func (m *MemStore) UpdateRelation(
	ctx context.Context, from, relType, to string, data store.RelationData,
) (*entity.Relation, error) {
	defer m.lockTx()()
	return m.updateRelation(ctx, from, relType, to, data)
}

// DeleteRelation implements store.RelationWriter.
func (m *MemStore) DeleteRelation(ctx context.Context, from, relType, to string) error {
	defer m.lockTx()()
	return m.deleteRelation(ctx, from, relType, to)
}

// DeleteRelationState implements store.RelationWriter.
func (m *MemStore) DeleteRelationState(
	ctx context.Context, from string, p entity.Face, relType, to string,
) error {
	defer m.lockTx()()
	return m.deleteRelationState(ctx, from, p, relType, to)
}

// AttachFile implements store.AttachmentManager.
func (m *MemStore) AttachFile(ctx context.Context, entityID, property, fileName string, r io.Reader) error {
	defer m.lockTx()()
	return m.attachFile(ctx, entityID, property, fileName, r)
}

// DeleteAttachment implements store.AttachmentManager.
func (m *MemStore) DeleteAttachment(ctx context.Context, entityID, property, fileName string) error {
	defer m.lockTx()()
	return m.deleteAttachment(ctx, entityID, property, fileName)
}

// --- transaction view ---

// txStore is the view a Tx callback receives: write methods skip txMu
// (the Tx already holds it) and call the cores directly; everything
// else is promoted from the embedded *MemStore. A nested Tx joins the
// open transaction.
type txStore struct{ *MemStore }

// interface check: the view must satisfy the full store.Store.
var _ store.Store = txStore{}

func (t txStore) CreateEntity(ctx context.Context, e *entity.Entity) error {
	return t.createEntity(ctx, e)
}

func (t txStore) UpdateEntity(ctx context.Context, e *entity.Entity) error {
	return t.updateEntity(ctx, e)
}

func (t txStore) DeleteEntity(ctx context.Context, id string, cascade bool) (*store.DeleteResult, error) {
	return t.deleteEntity(ctx, id, cascade)
}

func (t txStore) DeleteEntityState(
	ctx context.Context, id string, p entity.Face,
) (*store.DeleteResult, error) {
	return t.deleteEntityState(ctx, id, p)
}

func (t txStore) RenameEntity(ctx context.Context, oldID, newID string) (*store.RenameResult, error) {
	return t.renameEntity(ctx, oldID, newID)
}

func (t txStore) CreateRelation(
	ctx context.Context, from, relType, to string, data *store.RelationData,
) (*entity.Relation, error) {
	return t.createRelation(ctx, from, relType, to, data)
}

func (t txStore) UpdateRelation(
	ctx context.Context, from, relType, to string, data store.RelationData,
) (*entity.Relation, error) {
	return t.updateRelation(ctx, from, relType, to, data)
}

func (t txStore) DeleteRelation(ctx context.Context, from, relType, to string) error {
	return t.deleteRelation(ctx, from, relType, to)
}

func (t txStore) DeleteRelationState(
	ctx context.Context, from string, p entity.Face, relType, to string,
) error {
	return t.deleteRelationState(ctx, from, p, relType, to)
}

func (t txStore) AttachFile(ctx context.Context, entityID, property, fileName string, r io.Reader) error {
	return t.attachFile(ctx, entityID, property, fileName, r)
}

func (t txStore) DeleteAttachment(ctx context.Context, entityID, property, fileName string) error {
	return t.deleteAttachment(ctx, entityID, property, fileName)
}

// Tx on the view joins the open transaction.
func (t txStore) Tx(_ context.Context, fn func(store.Store) error) error {
	return fn(t)
}
