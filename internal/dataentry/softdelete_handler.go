package dataentry

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// softDeleter is what the web DELETE and the restore endpoint need to back
// the Undo toast. A plain DELETE of a whole entity soft-deletes it, so the
// toast can undo it with POST /{plural}/{id}/restore until the soft-delete GC
// purges it (see appbuild's startSoftDeleteGC).
//
// Nil when the store cannot soft-delete: DELETE then hard-deletes, and the
// restore route answers 404.
type softDeleter interface {
	SoftDeleteEntity(ctx context.Context, id string) (*entityPkg.DeleteResult, error)
	FindSoftDeleted(ctx context.Context, id string) (entitymanager.SoftDeleted, bool, error)
	RestoreEntity(ctx context.Context, id string) (*entityPkg.DeleteResult, error)
}

// softDeletesFor returns em's soft-delete capability, or nil when its store
// has none.
func softDeletesFor(em *entitymanager.Manager) softDeleter {
	if em == nil || !entitymanager.SupportsSoftDelete(em) {
		return nil
	}
	return entitymanager.SoftDeletes{M: em}
}

// deleteWholeEntity deletes the entity id with its relations: a soft delete
// when the store supports it, otherwise the hard cascade delete.
func (h *writeHandler) deleteWholeEntity(ctx context.Context, id string) error {
	if h.softDeletes != nil {
		_, err := h.softDeletes.SoftDeleteEntity(ctx, id)
		return err
	}
	_, err := h.manager.DeleteEntity(ctx, id, true)
	return err
}

// handleV1RestoreEntity serves POST /{plural}/{id}/restore: it undoes a soft
// delete.
//
// Read gate: whether a soft-deleted entity exists is as secret as whether a
// live one does, so the caller must either pass the normal read gate or be
// the principal who deleted it. Anyone else gets the same 404 as for an id
// that was never deleted. The deleter exception is what makes Undo work for a
// user whose read access came only from the entity's own relations, which are
// hidden while it is deleted. It reveals nothing: the deleter saw the entity
// a moment ago.
//
// Write authorization is the manager's: a restore needs the delete grants the
// delete needed, on the entity and on each relation that comes back (see
// [entitymanager.RestoreEntity]), so a denial is a 403.
func (h *writeHandler) handleV1RestoreEntity(w http.ResponseWriter, r *http.Request, typeName, entityID string) {
	if r.Method != http.MethodPost {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}
	r = h.withProvision(r)
	ctx := r.Context()

	// Only a whole entity is soft-deleted, so only a bare id can be restored.
	addr, err := entityPkg.ParseAddress(entityID)
	_, named := addr.Named()
	if err != nil || named || h.softDeletes == nil {
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
		return
	}

	found, marked, err := h.softDeletes.FindSoftDeleted(ctx, addr.ID())
	if err != nil {
		slog.Error("dataentry: restore lookup failed", "err", err)
		writeV1Error(w, r, http.StatusInternalServerError, "restore_failed", "Failed to restore entity", "")
		return
	}
	// The gate runs whether or not the id is marked, so a hidden mark and a
	// missing one cost the same. It runs under the reveal for the reason
	// RestoreEntity authorizes under it: local roles come from the entity's
	// own relations.
	readable, err := readGateFromContext(ctx).PermitsRead(store.WithRevealed(ctx, addr.ID()), typeName, addr.ID())
	if err != nil {
		writeGateError(w, r, err)
		return
	}
	user := principal.From(ctx).User
	isDeleter := user != principal.Unknown && user == found.DeletedBy
	mayKnow := readable || isDeleter
	if !marked || found.Type != typeName || !mayKnow {
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
		return
	}

	if _, err := h.softDeletes.RestoreEntity(ctx, addr.ID()); err != nil {
		h.writeRestoreError(w, r, err)
		return
	}
	// The store emits create events for the restored rows, so the SSE bridge
	// tells every browser. The client refetches the entity it restored.
	w.WriteHeader(http.StatusNoContent)
}

func (h *writeHandler) writeRestoreError(w http.ResponseWriter, r *http.Request, err error) {
	if writeForbiddenIfACLDenied(w, err) {
		return
	}
	if errors.Is(err, entitymanager.ErrEntityNotFound) {
		// Purged or restored by someone else since the lookup.
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
		return
	}
	var verr *entitymanager.ValidationError
	if errors.As(err, &verr) {
		// A live entity took a unique value meanwhile. Soft-deleted entities
		// hold their values, so this needs a racing non-manager write.
		writeV1Error(w, r, http.StatusConflict, "restore_conflict", "Entity cannot be restored", verr.Error())
		return
	}
	slog.Error("dataentry: restore failed", "err", err)
	writeV1Error(w, r, http.StatusInternalServerError, "restore_failed", "Failed to restore entity", "")
}
