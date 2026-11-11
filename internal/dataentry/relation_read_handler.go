package dataentry

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/canonical"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// handleV1GetRelationTarget serves a SINGLE relation's body — meta + content +
// the `_redacted` names — with a relation-level ETag (RR-SYNCR1, TKT-8P1TM7).
//
// It is the GET of the single-relation resource that PATCH and DELETE also
// address: the relation-type listing returns peer rows keyed to a source
// entity and carries no relation body or per-relation hash. It was added for
// the sync client (since removed) and stays as part of the public v1 API.
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
// gate resolved. The ETag hashes the relation as this caller sees it, with
// hidden meta removed: a digest over a hidden value would let the caller
// guess it offline, and a 304 would reveal when it changed (BUG-J01V0Q).
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

	view, etag := relationView(ctx, a.affordances, src, rel)
	w.Header().Set("ETag", etag)
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	redacted := redactedRelationKeys(rel.Properties, view.Properties)
	writeV1JSON(w, http.StatusOK, relationReadResponse{
		From:     src.ID,
		Type:     relType,
		To:       targetID,
		Meta:     view.Properties,
		Content:  rel.Content,
		Redacted: redacted,
		Updated:  updatedWhenWhole(rel.UpdatedAt, len(*redacted) == 0),
	})
}

// relationView is rel as the caller sees it, meta keys the caller may not
// read removed, and the ETag over that view. src is the edge's source row,
// whose grants decide the meta redaction.
//
// Redaction comes BEFORE hashing, and so before any 304 or If-Match compare:
// the validator is over the caller's view only (see [canonical.Digest]). A
// digest over a hidden value would let the caller guess it offline.
func relationView(
	ctx context.Context, aff affordanceService, src *entity.Entity, rel *entity.Relation,
) (view entity.Relation, etag string) {
	view = *rel
	view.Properties = aff.visibleRelationMeta(ctx, src, rel.Type, rel.Properties)
	return view, viewETag(canonical.SumRelation(view), view.Content, nil)
}

// updatedWhenWhole returns the last-updated time for the `_updated` wire
// field in RFC 3339, or "" when the caller does not see the whole row. The
// time says when ANY field last changed, so serving it next to a hidden field
// would tell the caller when that field may have changed. A zero time (a
// backend that keeps none) is "" too.
func updatedWhenWhole(t time.Time, whole bool) string {
	if !whole || t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
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
	if err != nil || !ref.Face.IsImplicit() || metamodel.IsContentScoped(meta, relType) {
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

// relationReadResponse is the single-relation read wire shape
// (v1RelationResponse mirrors its meta/content/_redacted fields).
type relationReadResponse struct {
	From     string         `json:"from"`
	Type     string         `json:"type"`
	To       string         `json:"to"`
	Meta     map[string]any `json:"meta,omitempty"`
	Content  string         `json:"content,omitempty"`
	Redacted *[]string      `json:"_redacted,omitempty"`
	// Updated is when the relation last changed, served only when no meta
	// key is hidden from the caller (see updatedWhenWhole).
	Updated string `json:"_updated,omitempty"`
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
