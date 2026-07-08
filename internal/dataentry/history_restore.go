package dataentry

import (
	"errors"
	"maps"
	"net/http"
	"reflect"
	"strconv"

	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// restoreHistoryVersion restores an entity's content + properties to a past
// version. It is modeled as "a PATCH whose body is the historical values":
// the diff between the live entity and the snapshot is run through
// validateFieldWrite BEFORE applying, so a principal cannot use restore to set
// or clear a field they lack write access to (RR-VOYXRV) — the whole reason
// restore is not a raw snapshot replay through UpdateEntity.
//
// If the entity currently exists it is updated; if it was deleted it is
// re-created. The restore is itself a normal write: authorized, validated,
// audited, and captured as a new version.
//
// Scope: entity content + properties only. The entity's relation set as-of the
// version is NOT restored (relation history is a separate capability).
func (a *App) restoreHistoryVersion(
	w http.ResponseWriter, r *http.Request, reader store.HistoryReader,
	typeName, entityID, versionStr string,
) {
	version, convErr := strconv.Atoi(versionStr)
	if convErr != nil || version < 1 {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_version",
			"Version must be a positive integer", "")
		return
	}

	ctx := r.Context()
	snap, err := reader.GetVersion(ctx, entityID, version)
	if errors.Is(err, store.ErrNotFound) {
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
		return
	}
	if err != nil {
		writeGateError(w, r, err)
		return
	}

	live, isLive := a.reader.getEntity(ctx, entityID)
	if isLive {
		a.restoreOntoLive(w, r, live, snap, typeName)
		return
	}
	a.restoreRecreate(w, r, snap, entityID)
}

// restoreOntoLive applies the snapshot onto an existing entity as a
// field-validated update: it computes which properties change (set) or are
// removed (unset) and which content changes, gates that exact set through
// validateFieldWrite, then applies via the entitymanager (which re-authorizes,
// validates, and audits).
func (a *App) restoreOntoLive(
	w http.ResponseWriter, r *http.Request, live *entityPkg.Entity, snap *store.VersionSnapshot, typeName string,
) {
	ctx := r.Context()

	// Property diff: keys the snapshot sets to a new/changed value, and keys the
	// live entity has that the snapshot does not (to be unset).
	setKeys := make(map[string]interface{})
	for k, v := range snap.Properties {
		if cur, ok := live.Properties[k]; !ok || !reflect.DeepEqual(cur, v) {
			setKeys[k] = v
		}
	}
	var unsetKeys []string
	for k := range live.Properties {
		if _, ok := snap.Properties[k]; !ok {
			unsetKeys = append(unsetKeys, k)
		}
	}

	// Gate exactly the fields that would change — same chokepoint a PATCH uses
	// (RR-VOYXRV). A field the principal can't write blocks the restore rather
	// than silently applying.
	if denial := a.affordances.validateFieldWrite(ctx, live, setKeys, unsetKeys); denial != nil {
		a.denyAffordance(ctx, w, live, *denial)
		return
	}

	// Build the target: live entity with the snapshot's content + properties.
	target := entityPkg.New(live.ID, live.Type)
	target.Content = snap.Content
	target.Properties = cloneProps(snap.Properties)

	if _, err := a.entityManager.UpdateEntity(ctx, target); err != nil {
		if writeForbiddenIfACLDenied(w, err) {
			return
		}
		if errors.Is(err, store.ErrNotFound) {
			writeV1Error(w, r, http.StatusConflict, "state_changed",
				"Entity state changed during restore (deleted concurrently) — retry", "")
			return
		}
		writeV1Error(w, r, http.StatusUnprocessableEntity, "validation_failed",
			"Restore failed validation", err.Error())
		return
	}
	a.writeRestoreResult(w, r, target, snap.Version, typeName)
}

// restoreRecreate re-creates a deleted entity from the snapshot. Create has no
// per-field write gate by design (create implies authoring the whole entity),
// so this is a type-level-authorized create through the entitymanager.
func (a *App) restoreRecreate(
	w http.ResponseWriter, r *http.Request, snap *store.VersionSnapshot, entityID string,
) {
	ctx := r.Context()
	target := entityPkg.New(entityID, snap.Type)
	target.Content = snap.Content
	target.Properties = cloneProps(snap.Properties)

	if _, err := a.entityManager.CreateEntity(ctx, target, entityPkg.CreateOptions{}); err != nil {
		if writeForbiddenIfACLDenied(w, err) {
			return
		}
		if errors.Is(err, store.ErrConflict) {
			writeV1Error(w, r, http.StatusConflict, "state_changed",
				"Entity state changed during restore (re-created concurrently) — retry", "")
			return
		}
		writeV1Error(w, r, http.StatusUnprocessableEntity, "validation_failed",
			"Restore failed validation", err.Error())
		return
	}
	a.writeRestoreResult(w, r, target, snap.Version, snap.Type)
}

func (a *App) writeRestoreResult(
	w http.ResponseWriter, r *http.Request, e *entityPkg.Entity, restoredFrom int, typeName string,
) {
	meta := a.Meta()
	plural := typeName
	if def, ok := meta.GetEntityDef(e.Type); ok {
		plural = def.GetPlural(e.Type)
	}
	wire := a.serializer.forWire(r.Context(), e, a.reader.outgoingRelations(r.Context(), e.ID), meta, plural)
	writeV1JSON(w, http.StatusOK, map[string]any{
		"restored_from_version": restoredFrom,
		"entity":                wire,
	})
}

// cloneProps returns a shallow copy so the entity handed to the write path does
// not alias the snapshot's map.
func cloneProps(p map[string]interface{}) map[string]interface{} {
	if p == nil {
		return nil
	}
	out := make(map[string]interface{}, len(p))
	maps.Copy(out, p)
	return out
}
