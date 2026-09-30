package dataentry

import (
	"errors"
	"net/http"
	"sort"

	"github.com/Sourcehaven-BV/rela/internal/canonical"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// handleV1GetRelationTarget serves a SINGLE relation's body — meta + content +
// the `_redacted` names — with a relation-level ETag (RR-SYNCR1, TKT-8P1TM7).
//
// It exists so the sync client can fetch a relation through the authorized
// /api/v1 read path (retiring the parallel /api/sync relation GET): the SPA's
// relation-type listing returns peer rows keyed to a source entity and carries
// no relation body or per-relation hash, which a faithful replica needs.
//
// Authorization gates BOTH endpoints, each on its stored type, never the URL
// segment (RR-SDDYZO). The tail is gated per relationTailOr404: a
// content-scoped edge tailed on a face the caller may not read is a 404
// (design 8.2). The head is named by bare id, so the check is that some
// face of it is readable. The head is gated BEFORE the edge is loaded: the
// edge-miss 404 carries a different title, so gating after would tell a
// caller whether an edge to a hidden entity exists.
//
// Field meta redaction reuses visibleRelationMeta against the source row the
// gate resolved. The ETag is over the RAW relation (canonical.HashRelation),
// never the redacted body, so it is a stable If-Match token independent of
// the reader's field visibility (the entity-side RR-IWXMDW invariant, applied
// to relations).
//
// It is a package function taking *App (not an App method) so it does not add
// to App's god-object method count (plimsoll, TKT-N0IKN9).
func handleV1GetRelationTarget(
	a *App, w http.ResponseWriter, r *http.Request, typeName, addr, relType, targetID string,
) {
	ctx := r.Context()

	mm := a.schema.Current().Meta
	src, ok := relationTailOr404(w, r, a.visibleReader, mm, typeName, addr, relType)
	if !ok {
		return
	}
	if !familyReadableOr404(w, r, a.visibleReader, targetID) {
		return
	}

	// The edge at the ADDRESSED TAIL. store.GetRelation reads the default
	// tail only, so on a faced source it would miss the edge entirely, or
	// return a different face's (BUG-VFHUWO). An identity-scoped edge
	// attaches to the entity, so its tail is always the zero face, whichever
	// face of the source the address resolved to.
	tail := src.Face
	if !metamodel.IsContentScoped(mm, relType) {
		tail = ""
	}
	rel, err := a.reader.store.GetRelation(ctx, entity.RelationKey{
		From: src.ID, FromFace: tail, Type: relType, To: targetID,
	})
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

	meta := a.affordances.visibleRelationMeta(ctx, src, relType, rel.Properties)
	writeV1JSON(w, http.StatusOK, relationReadResponse{
		From:     src.ID,
		Type:     relType,
		To:       targetID,
		Meta:     meta,
		Content:  rel.Content,
		Redacted: redactedRelationKeys(rel.Properties, meta),
	})
}

// relationTailOr404 reads the source row of a single-relation read (design
// 8.2). A content-scoped edge, or any address naming a face, is face level:
// the addressed row must be readable. An identity-scoped edge named by bare id
// is entity level: the row the request's world selects when it is readable,
// otherwise the first readable face, so a faced source with no zero-face row
// still answers. The returned row decides field redaction only; the edge is
// always read at the zero tail.
func relationTailOr404(
	w http.ResponseWriter, r *http.Request, vr visibleReader, meta *metamodel.Metamodel,
	typeName, addr, relType string,
) (*entity.Entity, bool) {
	ref, err := entity.ParseRef(addr)
	if err != nil || !ref.Face.IsDefault() || metamodel.IsContentScoped(meta, relType) {
		return readAddressedOr404(w, r, vr, typeName, addr)
	}
	ctx := r.Context()
	src, found, err := vr.inWorld(ctx, typeName, ref.ID)
	if err == nil && !found {
		var fam visibility.Family
		if fam, found, err = vr.family(ctx, typeName, ref.ID); err == nil && found {
			src, found, err = vr.ref(ctx, typeName, entity.Ref{ID: ref.ID, Face: fam.Faces[0]})
		}
	}
	if err != nil {
		writeGateError(w, r, err)
		return nil, false
	}
	if !found {
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
		return nil, false
	}
	return src, true
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
