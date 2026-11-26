package dataentry

import (
	"errors"
	"maps"
	"net/http"
	"reflect"
	"strconv"

	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// restoreHistoryVersion restores an entity's content and properties to a past
// version, as a PATCH whose body is the historical values: the diff is checked
// with validateFieldWrite first, so restore cannot set or clear a field the
// principal may not write. A live entity is updated, a deleted one is
// re-created, and the restore is a normal authorized, audited write.
// The snapshot's type must match the URL type. Relations and file values are
// not restored; live file values are kept.
func restoreHistoryVersion(a *App,
	w http.ResponseWriter, r *http.Request, reader store.HistoryReader,
	typeName string, subject historySubject, versionStr string,
) {
	version, convErr := strconv.Atoi(versionStr)
	if convErr != nil || version < 1 {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_version",
			"Version must be a positive integer", "")
		return
	}

	ctx := r.Context()
	snap, err := reader.GetVersion(ctx, subject.ref, version)
	if errors.Is(err, store.ErrNotFound) {
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
		return
	}
	if err != nil {
		writeGateError(w, r, err)
		return
	}
	if snap.Type != typeName || snap.Face != subject.ref.Face {
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
		return
	}

	if subject.live != nil {
		restoreOntoLive(a, w, r, subject.live, snap, typeName)
		return
	}
	restoreRecreate(a, w, r, snap, subject.ref)
}

// restoreOntoLive applies the snapshot onto an existing entity as a
// field-validated update: it computes which properties change (set) or are
// removed (unset) and which content changes, gates that exact set through
// validateFieldWrite, then patches that set through the manager's update grant
// (authorized once, validated and audited).
func restoreOntoLive(a *App,
	w http.ResponseWriter, r *http.Request, live *entityPkg.Entity, snap *store.VersionSnapshot, typeName string,
) {
	ctx := r.Context()
	props := entitymanager.CarryFileValues(a.Meta(), typeName, snap.Properties, live)

	setKeys := make(map[string]any)
	for k, v := range props {
		if cur, ok := live.Properties[k]; !ok || !reflect.DeepEqual(cur, v) {
			setKeys[k] = v
		}
	}
	var unsetKeys []string
	for k := range live.Properties {
		if _, ok := props[k]; !ok {
			unsetKeys = append(unsetKeys, k)
		}
	}

	if denial := a.affordances.validateFieldWrite(ctx, live, setKeys, unsetKeys); denial != nil {
		a.denyAffordance(ctx, w, live, *denial)
		return
	}

	grant, err := a.write.manager.AuthorizeUpdate(ctx, live)
	if err != nil {
		writeRestoreWriteError(w, r, err)
		return
	}
	content := snap.Content
	patch := entityPkg.Patch{Properties: setKeys, MetaUnset: unsetKeys, Content: &content}
	if _, err := a.write.manager.PatchGranted(ctx, grant, live.Ref().String(), patch); err != nil {
		writeRestoreWriteError(w, r, err)
		return
	}

	target := entityPkg.New(live.ID, live.Type)
	target.Face = live.Face
	target.Content = snap.Content
	target.Properties = props
	writeRestoreResult(a, w, r, target, snap.Version, typeName)
}

// restoreRecreate re-creates a deleted entity from the snapshot. Unlike a
// normal create — where the author types the values — a restore's values come
// from a historical snapshot the principal may never have been allowed to
// write. So every snapshot property is gated through validateCreateFieldWrite
// (as if the principal typed it into a create), like restoreOntoLive: otherwise
// resurrection would launder a forbidden field write (RR-LH9RJ8). Type-level
// create authorization + validation still run in the entitymanager.
func restoreRecreate(a *App,
	w http.ResponseWriter, r *http.Request, snap *store.VersionSnapshot, ref entityPkg.Ref,
) {
	ctx := r.Context()
	target := entityPkg.New(ref.ID, snap.Type)
	target.Face = ref.Face
	target.Content = snap.Content
	target.Properties = entitymanager.CarryFileValues(a.Meta(), snap.Type, snap.Properties, nil)

	if denial := a.affordances.validateCreateFieldWrite(ctx, target, target.Properties); denial != nil {
		a.denyAffordance(ctx, w, target, *denial)
		return
	}

	if _, err := a.recreator.RecreateEntity(ctx, target); err != nil {
		writeRestoreWriteError(w, r, err)
		return
	}
	writeRestoreResult(a, w, r, target, snap.Version, snap.Type)
}

// writeRestoreWriteError maps a failed restore write. A row that appeared or
// vanished since the restore read the address is a 409 the client retries; a
// face error is about the address, as on a create; anything else is the
// write path's 422, as on every other entity write.
func writeRestoreWriteError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case writeForbiddenIfACLDenied(w, r, err), writeFieldWriteRefused(w, err):
	case errors.Is(err, entitymanager.ErrEntityAlreadyExists),
		errors.Is(err, entitymanager.ErrEntityNotFound),
		errors.Is(err, store.ErrNotFound):
		writeV1Error(w, r, http.StatusConflict, "state_changed",
			"Entity state changed during restore; retry", "")
	case errors.Is(err, entitymanager.ErrFaceRequired), errors.Is(err, entitymanager.ErrFaceNotDeclared):
		writeV1Error(w, r, http.StatusUnprocessableEntity, "face_required", err.Error(), "")
	default:
		writeV1Error(w, r, http.StatusUnprocessableEntity, "validation_failed",
			"Restore failed validation", err.Error())
	}
}

func writeRestoreResult(a *App,
	w http.ResponseWriter, r *http.Request, e *entityPkg.Entity, restoredFrom int, typeName string,
) {
	meta := a.Meta()
	plural := typeName
	if def, ok := meta.GetEntityDef(e.Type); ok {
		plural = def.GetPlural(e.Type)
	}
	rels, visibleNeighbors, rerr := a.faceEdges.read(r.Context(), e)
	if rerr != nil {
		writeGateError(w, r, rerr)
		return
	}
	wire := a.serializer.forWireScoped(r.Context(), e, rels, visibleNeighbors, meta, plural)
	writeV1JSON(w, http.StatusOK, map[string]any{
		"restored_from_version": restoredFrom,
		"entity":                wire,
	})
}

// cloneProps returns a shallow copy so the entity handed to the write path does
// not alias the snapshot's map.
func cloneProps(p map[string]any) map[string]any {
	if p == nil {
		return nil
	}
	out := make(map[string]any, len(p))
	maps.Copy(out, p)
	return out
}
