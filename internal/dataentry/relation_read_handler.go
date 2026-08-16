package dataentry

import (
	"errors"
	"net/http"
	"sort"

	"github.com/Sourcehaven-BV/rela/internal/canonical"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// handleV1GetRelationTarget serves a SINGLE relation's body — meta + content +
// the `_redacted` names — with a relation-level ETag (RR-SYNCR1, TKT-8P1TM7).
//
// It exists so the sync client can fetch a relation through the authorized
// /api/v1 read path (retiring the parallel /api/sync relation GET): the SPA's
// relation-type listing returns peer rows keyed to a source entity and carries
// no relation body or per-relation hash, which a faithful replica needs.
//
// Authorization mirrors the relation-history read (RR-SDDYZO): BOTH endpoints
// must be readable, each gated on its LIVE type (never the URL segment). Field
// meta redaction reuses visibleRelationMeta and FAILS CLOSED — if the source
// endpoint is not live, no meta reaches the wire. The ETag is over the RAW
// relation (canonical.HashRelation), never the redacted body, so it is a stable
// If-Match token independent of the reader's field visibility (the entity-side
// RR-IWXMDW invariant, applied to relations).
// It is a package function taking *App (not an App method) so it does not add to
// App's god-object method count — the pattern App's own doc records for
// receiver-free handler helpers (plimsoll, TKT-N0IKN9).
func handleV1GetRelationTarget(
	a *App, w http.ResponseWriter, r *http.Request, typeName, entityID, relType, targetID string,
) {
	ctx := r.Context()

	// The path entity must exist and match the route type, or it is an
	// indistinguishable 404 (same as a get on the wrong-typed id).
	if src, ok := a.reader.getEntity(ctx, entityID); !ok || src.Type != typeName {
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
		return
	}

	// Dual-endpoint gate: from = entityID (the path entity), to = targetID.
	if !authorizeRelationEndpointsReadable(a, w, r, entityID, targetID) {
		return
	}

	rel, err := a.reader.store.GetRelation(ctx, entityID, relType, targetID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeV1Error(w, r, http.StatusNotFound, "not_found", "Relation not found", "")
			return
		}
		writeGateError(w, r, err)
		return
	}

	// ETag over the RAW relation (reader-independent → lossless If-Match).
	etag := canonical.HashRelation(*rel)
	w.Header().Set("ETag", etag)
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	// Redact meta, fail-closed: resolve the live SOURCE entity; if it is gone,
	// emit no meta rather than raw meta (mirrors the relation-history handler).
	source, live := a.reader.getEntity(ctx, entityID)
	var meta map[string]any
	if live {
		meta = a.affordances.visibleRelationMeta(ctx, source, relType, rel.Properties)
	}

	writeV1JSON(w, http.StatusOK, relationReadResponse{
		From:     entityID,
		Type:     relType,
		To:       targetID,
		Meta:     meta,
		Content:  rel.Content,
		Redacted: redactedRelationKeys(rel.Properties, meta),
	})
}

// relationReadResponse is the single-relation read wire shape the sync client
// decodes (v1RelationResponse mirrors its meta/content/_redacted fields).
type relationReadResponse struct {
	From     string         `json:"from"`
	Type     string         `json:"type"`
	To       string         `json:"to"`
	Meta     map[string]any `json:"meta,omitempty"`
	Content  string         `json:"content,omitempty"`
	Redacted *[]string      `json:"_redacted,omitempty"`
}

// authorizeRelationEndpointsReadable gates a single-relation read on BOTH
// endpoints being live and readable, each on its real (live) type. A denial on
// either endpoint is an indistinguishable 404. A not-live endpoint means the
// relation's endpoints are (partly) gone — for a live-relation read that is the
// same 404 (the sync read is a live-world read, unlike history which has the
// deleted-relation PermHistoryRead path).
func authorizeRelationEndpointsReadable(
	a *App, w http.ResponseWriter, r *http.Request, from, to string,
) bool {
	ctx := r.Context()
	gate := readGateFromContext(ctx)

	fromEntity, fromLive := a.reader.getEntity(ctx, from)
	toEntity, toLive := a.reader.getEntity(ctx, to)
	if !fromLive || !toLive {
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
		return false
	}

	fromOK, err := gate.PermitsRead(ctx, fromEntity.Type, from)
	if err != nil {
		writeGateError(w, r, err)
		return false
	}
	toOK, err := gate.PermitsRead(ctx, toEntity.Type, to)
	if err != nil {
		writeGateError(w, r, err)
		return false
	}
	if !fromOK || !toOK {
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
		return false
	}
	return true
}

// redactedRelationKeys returns the sorted names of meta keys present in the raw
// relation but withheld from the redacted meta, for the `_redacted` wire field
// (the relation analog of redactedPropertyNames). It is always non-nil so the
// replica can distinguish "hidden" (named here) from "deleted" (absent from both
// meta and _redacted) — the TKT-8P1TM7 splice contract. Names are not secret.
func redactedRelationKeys(raw, visible map[string]any) *[]string {
	out := make([]string, 0)
	for k := range raw {
		if _, ok := visible[k]; !ok {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return &out
}
