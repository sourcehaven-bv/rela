package dataentry

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/canonical"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// syncContext re-stamps the request principal's Tool as ToolSync, preserving the
// proxy-set User, so a synced write is audited as Tool=sync rather than
// data-entry. Reads the User from the principal stampAuditPrincipal already set.
func syncContext(ctx context.Context) context.Context {
	p := principal.From(ctx)
	return principal.With(ctx, principal.Principal{User: p.User, Tool: principal.ToolSync})
}

// handleSyncGet fetches a record's full content as JSON, plus its current hash
// in the ETag header so the client can use it as an If-Match base on a later
// push.
func (a *App) handleSyncGet(w http.ResponseWriter, r *http.Request, kind, rest string) {
	switch kind {
	case "entities":
		if !validIDSegment(rest) {
			writeV1Error(w, r, http.StatusBadRequest, "invalid_id", "Invalid entity id", "")
			return
		}
		e, err := a.store.GetEntity(r.Context(), rest)
		if err != nil {
			writeV1Error(w, r, http.StatusNotFound, "not_found", "Entity not found", "")
			return
		}
		// ACL read gate: the sync GET reads via store directly, so it must
		// apply the same read authorization every other read path does
		// (RR IB-review #1). A denied read 404s indistinguishably from a
		// missing one — same body as the err branch above.
		if ok, err := a.permitsSyncReadEntity(r.Context(), e.Type, e.ID); err != nil {
			writeGateError(w, r, err)
			return
		} else if !ok {
			writeV1Error(w, r, http.StatusNotFound, "not_found", "Entity not found", "")
			return
		}
		w.Header().Set("ETag", canonical.HashEntity(*e))
		writeV1JSON(w, http.StatusOK, syncEntityBody{
			ID: e.ID, Type: e.Type, Properties: e.Properties, Content: e.Content,
		})
	case "relations":
		rel, ok := a.fetchRelation(w, r, rest)
		if !ok {
			return
		}
		// A relation's read visibility follows its source entity, exactly
		// as handleV1EntityRelations gates /relations (api_v1.go).
		if ok, err := a.permitsSyncReadRelation(r.Context(), rel.From); err != nil {
			writeGateError(w, r, err)
			return
		} else if !ok {
			writeV1Error(w, r, http.StatusNotFound, "not_found", "Relation not found", "")
			return
		}
		w.Header().Set("ETag", canonical.HashRelation(*rel))
		writeV1JSON(w, http.StatusOK, syncRelationBody{
			From: rel.From, Type: rel.Type, To: rel.To, Properties: rel.Properties, Content: rel.Content,
		})
	}
}

// handleSyncPut is the conditional push. It applies the record via the
// id-preserving, automation-suppressed sync apply path, gated by If-Match:
//
//	200 + new ETag : applied (the record's current hash matched If-Match, or
//	                 If-Match was absent for a first create)
//	412            : If-Match did not match the record's current hash → the
//	                 remote moved since the client's base → conflict
//	422            : entitymanager rejected the content (validation) — NOT a
//	                 conflict; the data is invalid
//	403            : ACL denied
func (a *App) handleSyncPut(w http.ResponseWriter, r *http.Request, kind, rest string) {
	ap := a.syncApplierFor()
	if ap == nil {
		writeV1Error(w, r, http.StatusNotImplemented, "sync_unsupported", "Sync apply is not wired", "")
		return
	}
	ifMatch := strings.TrimSpace(r.Header.Get("If-Match"))

	a.writeMu.Lock()
	defer a.writeMu.Unlock()

	switch kind {
	case "entities":
		a.putEntity(w, r, ap, rest, ifMatch)
	case "relations":
		a.putRelation(w, r, ap, rest, ifMatch)
	}
}

func (a *App) putEntity(w http.ResponseWriter, r *http.Request, ap syncApplier, id, ifMatch string) {
	if !validIDSegment(id) {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_id", "Invalid entity id", "")
		return
	}
	var body syncEntityBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeV1Error(w, r, http.StatusBadRequest, "bad_request", "Malformed JSON body", "")
		return
	}
	if body.ID != "" && body.ID != id {
		writeV1Error(w, r, http.StatusBadRequest, "id_mismatch", "Body id does not match the path id", "")
		return
	}

	// Precondition: the record's CURRENT hash must equal If-Match. A push with
	// no If-Match is only valid if the record does not yet exist (a first
	// create); otherwise the client must declare the base it edited.
	cur, exists := a.currentEntityHash(r.Context(), id)
	if !preconditionOK(ifMatch, cur, exists) {
		writeSyncConflict(w, r, cur, exists)
		return
	}

	e := &entity.Entity{ID: id, Type: body.Type, Properties: body.Properties, Content: body.Content}
	if _, err := ap.ApplyEntity(syncContext(r.Context()), e); err != nil {
		writeSyncApplyError(w, r, err)
		return
	}
	w.Header().Set("ETag", canonical.HashEntity(*e))
	writeV1JSON(w, http.StatusOK, map[string]string{"hash": canonical.HashEntity(*e)})
}

func (a *App) putRelation(w http.ResponseWriter, r *http.Request, ap syncApplier, rest, ifMatch string) {
	from, relType, to, ok := parseRelationKey(rest)
	if !ok {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_id", "Invalid relation key", "")
		return
	}
	var body syncRelationBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeV1Error(w, r, http.StatusBadRequest, "bad_request", "Malformed JSON body", "")
		return
	}

	cur, exists := a.currentRelationHash(r.Context(), from, relType, to)
	if !preconditionOK(ifMatch, cur, exists) {
		writeSyncConflict(w, r, cur, exists)
		return
	}

	rel := &entity.Relation{From: from, Type: relType, To: to, Properties: body.Properties, Content: body.Content}
	if _, err := ap.ApplyRelation(syncContext(r.Context()), rel); err != nil {
		writeSyncApplyError(w, r, err)
		return
	}
	w.Header().Set("ETag", canonical.HashRelation(*rel))
	writeV1JSON(w, http.StatusOK, map[string]string{"hash": canonical.HashRelation(*rel)})
}

// handleSyncDelete is the conditional delete: If-Match MUST be present and equal
// the record's current hash — a client only deletes what it last saw, symmetric
// with the push precondition (no blind delete of an existing record). 200 on
// success, 412 on a missing/mismatched If-Match, 404 if the record is gone.
func (a *App) handleSyncDelete(w http.ResponseWriter, r *http.Request, kind, rest string) {
	a.writeMu.Lock()
	defer a.writeMu.Unlock()
	ifMatch := strings.TrimSpace(r.Header.Get("If-Match"))

	switch kind {
	case "entities":
		if !validIDSegment(rest) {
			writeV1Error(w, r, http.StatusBadRequest, "invalid_id", "Invalid entity id", "")
			return
		}
		cur, exists := a.currentEntityHash(r.Context(), rest)
		if !exists {
			writeV1Error(w, r, http.StatusNotFound, "not_found", "Entity not found", "")
			return
		}
		if !deletePreconditionOK(ifMatch, cur) {
			writeSyncConflict(w, r, cur, true)
			return
		}
		if _, err := a.entityManager.DeleteEntity(syncContext(r.Context()), rest, true); err != nil {
			writeSyncApplyError(w, r, err)
			return
		}
		writeV1JSON(w, http.StatusOK, map[string]string{"deleted": rest})
	case "relations":
		from, relType, to, ok := parseRelationKey(rest)
		if !ok {
			writeV1Error(w, r, http.StatusBadRequest, "invalid_id", "Invalid relation key", "")
			return
		}
		cur, exists := a.currentRelationHash(r.Context(), from, relType, to)
		if !exists {
			writeV1Error(w, r, http.StatusNotFound, "not_found", "Relation not found", "")
			return
		}
		if !deletePreconditionOK(ifMatch, cur) {
			writeSyncConflict(w, r, cur, true)
			return
		}
		if err := a.entityManager.DeleteRelation(syncContext(r.Context()), from, relType, to); err != nil {
			writeSyncApplyError(w, r, err)
			return
		}
		writeV1JSON(w, http.StatusOK, map[string]string{"deleted": rest})
	}
}

// deletePreconditionOK requires a non-empty If-Match that matches the current
// hash. Unlike a push (where a first create legitimately has no base), a delete
// always targets an existing record, so a missing If-Match is a precondition
// failure (412) — never a blind delete. Force-delete is the client's `--force`
// path, which re-reads the current hash and supplies it.
func deletePreconditionOK(ifMatch, currentHash string) bool {
	return ifMatch != "" && ifMatch == currentHash
}

// --- helpers ---

// permitsSyncReadEntity is the read-ACL probe for a sync entity read. It mirrors
// the per-entity gate every other read path uses (gateReadOrNotFound in
// api_v1.go): the answer is "policy permits reading this (type, id)", evaluated
// against the request principal's read scope. The nop gate (no ACL configured)
// permits everything, preserving pre-ACL behavior.
func (a *App) permitsSyncReadEntity(ctx context.Context, entityType, entityID string) (bool, error) {
	return readGateFromContext(ctx).PermitsRead(ctx, entityType, entityID)
}

// permitsSyncReadRelation is the read-ACL probe for a sync relation read. A
// relation carries no type of its own; its visibility follows the source
// (From) entity, exactly as handleV1EntityRelations gates /relations. The
// source entity's type is resolved from the store; if it cannot be loaded
// (e.g. the source was deleted) the type is left empty, the same fallback the
// relation write gate uses (authorizeConflictResolve), and the gate decides.
func (a *App) permitsSyncReadRelation(ctx context.Context, from string) (bool, error) {
	var fromType string
	if e, err := a.store.GetEntity(ctx, from); err == nil {
		fromType = e.Type
	}
	return readGateFromContext(ctx).PermitsRead(ctx, fromType, from)
}

// fetchRelation parses the relation key from rest, loads it, and writes a 4xx if
// the key is invalid or the relation is absent. Returns (rel, true) on success.
func (a *App) fetchRelation(w http.ResponseWriter, r *http.Request, rest string) (*entity.Relation, bool) {
	from, relType, to, ok := parseRelationKey(rest)
	if !ok {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_id", "Invalid relation key", "")
		return nil, false
	}
	rel, err := a.store.GetRelation(r.Context(), from, relType, to)
	if err != nil {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Relation not found", "")
		return nil, false
	}
	return rel, true
}

// parseRelationKey splits "<from>/<relType>/<to>" and allowlist-validates each
// segment.
func parseRelationKey(rest string) (from, relType, to string, ok bool) {
	parts := strings.Split(rest, "/")
	if len(parts) != 3 {
		return "", "", "", false
	}
	from, relType, to = parts[0], parts[1], parts[2]
	if !validIDSegment(from) || !validIDSegment(relType) || !validIDSegment(to) {
		return "", "", "", false
	}
	return from, relType, to, true
}

func (a *App) currentEntityHash(ctx context.Context, id string) (hash string, exists bool) {
	e, err := a.store.GetEntity(ctx, id)
	if err != nil {
		return "", false
	}
	return canonical.HashEntity(*e), true
}

func (a *App) currentRelationHash(ctx context.Context, from, relType, to string) (hash string, exists bool) {
	rel, err := a.store.GetRelation(ctx, from, relType, to)
	if err != nil {
		return "", false
	}
	return canonical.HashRelation(*rel), true
}

// preconditionOK reports whether a push/delete may proceed. With an If-Match
// header the record's current hash must equal it. With no If-Match the record
// must NOT already exist (a first create) — otherwise the client is pushing
// blind over an existing record and must declare its base.
func preconditionOK(ifMatch, currentHash string, exists bool) bool {
	if ifMatch == "" {
		return !exists
	}
	return exists && ifMatch == currentHash
}

// writeSyncConflict writes the 412 the client reads as "remote moved since your
// base — resolve the conflict". The current hash is returned in ETag so the
// client can re-baseline on a forced push.
func writeSyncConflict(w http.ResponseWriter, r *http.Request, currentHash string, exists bool) {
	if exists {
		w.Header().Set("ETag", currentHash)
	}
	writeV1Error(w, r, http.StatusPreconditionFailed, "conflict",
		"The record changed on the server since your base; resolve the conflict", "")
}

// writeSyncApplyError maps an apply/delete error to a status. A validation
// error is 422 (the content is invalid — NOT a conflict, which is 412). An ACL
// denial is 403. Everything else is 500.
func writeSyncApplyError(w http.ResponseWriter, r *http.Request, err error) {
	var valErr *entitymanager.ValidationError
	if errors.As(err, &valErr) {
		writeV1Error(w, r, http.StatusUnprocessableEntity, "validation_failed", "The content is invalid", err.Error())
		return
	}
	var forbidden *acl.ForbiddenError
	if errors.As(err, &forbidden) {
		writeV1Error(w, r, http.StatusForbidden, "forbidden", "Not permitted", "")
		return
	}
	if errors.Is(err, entitymanager.ErrEntityNotFound) {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Referenced record not found", "")
		return
	}
	writeV1Error(w, r, http.StatusInternalServerError, "apply_failed", "Failed to apply the change", "")
}
