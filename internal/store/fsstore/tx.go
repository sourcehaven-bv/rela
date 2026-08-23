package fsstore

import (
	"context"
	"io"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Tx and the txMu write serialization (store.Transactor, DEC-8UIL0).
//
// fsstore meets the Tx contract with the reduced single-user guarantees
// the decision allows for filesystem deployments: mutual exclusion only.
// Tx holds txMu for the whole callback; every exported write method
// takes txMu briefly before doing its normal work, so an open Tx
// excludes ordinary writers and vice versa. There is NO rollback (an
// error from fn leaves earlier writes applied — the filesystem cannot
// promise crash atomicity either) and events/observer notifications are
// emitted inline per write, exactly as outside a Tx. The watcher's
// reconciliation of external file edits is not excluded — hand edits
// were never serialized against API writes.
//
// Go has no reentrant mutex (deliberately), so re-entrancy is
// structural: fn receives a txStore view whose write methods call the
// unexported cores directly, skipping txMu. Calling a write on the
// OUTER store from inside fn deadlocks — use the view for everything.

// Tx implements store.Transactor. See the package notes above for the
// fsstore-specific (reduced) guarantees.
func (s *FSStore) Tx(_ context.Context, fn func(store.Store) error) error {
	s.txMu.Lock()
	defer s.txMu.Unlock()
	return fn(txStore{s})
}

// lockTx takes the Tx serialization lock and returns its release func,
// letting the exported write wrappers stay one-liners.
func (s *FSStore) lockTx() func() {
	s.txMu.Lock()
	return s.txMu.Unlock
}

// --- exported write API: serialize with any open Tx, then delegate ---

// CreateEntity implements store.EntityWriter.
// Returns store.ErrConflict if an entity with the same ID exists.
func (s *FSStore) CreateEntity(ctx context.Context, e *entity.Entity) error {
	defer s.lockTx()()
	return s.createEntity(ctx, e)
}

// UpdateEntity implements store.EntityWriter.
// Returns store.ErrNotFound if the entity does not exist.
func (s *FSStore) UpdateEntity(ctx context.Context, e *entity.Entity) error {
	defer s.lockTx()()
	return s.updateEntity(ctx, e)
}

// DeleteEntity implements store.EntityWriter.
// Returns store.ErrNotFound if the entity does not exist.
func (s *FSStore) DeleteEntity(ctx context.Context, id string, cascade bool) (*store.DeleteResult, error) {
	defer s.lockTx()()
	return s.deleteEntity(ctx, id, cascade)
}

// DeleteEntityState implements store.EntityWriter.
// Returns store.ErrNotFound if that face does not exist.
func (s *FSStore) DeleteEntityState(
	ctx context.Context, id string, p entity.Pointer,
) (*store.DeleteResult, error) {
	defer s.lockTx()()
	return s.deleteEntityState(ctx, id, p)
}

// RenameEntity implements store.EntityWriter.
// Returns store.ErrNotFound if oldID is absent, store.ErrConflict if
// newID exists.
func (s *FSStore) RenameEntity(ctx context.Context, oldID, newID string) (*store.RenameResult, error) {
	defer s.lockTx()()
	return s.renameEntity(ctx, oldID, newID)
}

// CreateRelation implements store.RelationWriter.
func (s *FSStore) CreateRelation(
	ctx context.Context, from, relType, to string, data *store.RelationData,
) (*entity.Relation, error) {
	defer s.lockTx()()
	return s.createRelation(ctx, from, relType, to, data)
}

// UpdateRelation implements store.RelationWriter.
func (s *FSStore) UpdateRelation(
	ctx context.Context, from, relType, to string, data store.RelationData,
) (*entity.Relation, error) {
	defer s.lockTx()()
	return s.updateRelation(ctx, from, relType, to, data)
}

// DeleteRelation implements store.RelationWriter.
func (s *FSStore) DeleteRelation(ctx context.Context, from, relType, to string) error {
	defer s.lockTx()()
	return s.deleteRelation(ctx, from, relType, to)
}

// DeleteRelationState implements store.RelationWriter.
func (s *FSStore) DeleteRelationState(
	ctx context.Context, from string, p entity.Pointer, relType, to string,
) error {
	defer s.lockTx()()
	return s.deleteRelationState(ctx, from, p, relType, to)
}

// AttachFile implements store.AttachmentManager.
func (s *FSStore) AttachFile(ctx context.Context, entityID, property, fileName string, r io.Reader) error {
	defer s.lockTx()()
	return s.attachFile(ctx, entityID, property, fileName, r)
}

// DeleteAttachment implements store.AttachmentManager.
func (s *FSStore) DeleteAttachment(ctx context.Context, entityID, property, fileName string) error {
	defer s.lockTx()()
	return s.deleteAttachment(ctx, entityID, property, fileName)
}

// --- transaction view ---

// txStore is the view a Tx callback receives: write methods skip txMu
// (the Tx already holds it) and call the cores directly; everything
// else is promoted from the embedded *FSStore. A nested Tx joins the
// open transaction.
type txStore struct{ *FSStore }

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
	ctx context.Context, id string, p entity.Pointer,
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
	ctx context.Context, from string, p entity.Pointer, relType, to string,
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
