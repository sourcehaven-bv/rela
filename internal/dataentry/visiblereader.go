package dataentry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	entitypkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// visibleReader is the ACL-bounded entity-read seam for the data-entry
// handlers. Every single-entity read it exposes goes through one
// [visibility.Resolver] (TKT-2528AB), so the world, the face-blind row gate,
// the readable-face set, the load, the stored-type check and the face gate
// run in one place and in one order. A handler cannot obtain a row from it
// without the face gate having run.
//
// The resolver is built over [ctxRowGate], which resolves the per-request
// gate from ctx at call time, so it keeps the binding [attachACLRequest] sets
// up. Under no ACL that is the permit-all [nopReadGate].
//
// # It returns raw rows, deliberately
//
// The resolver here redacts with [visibility.NopRedactor]. Field redaction on
// the data-entry surfaces happens once, where the response is built
// (stripHiddenProperties and `_redacted`), and [visibility.Redact] is not
// composable: redacting here as well would evaluate `visible:` conditions a
// second time, on an already-redacted row. The write preflights need the raw
// row too, because a redacted read-modify-write would clobber hidden fields.
//
// A miss is (nil, false, nil). It covers denied, missing, type-mismatched,
// face-denied, world-denied and a failed load, so a caller renders every miss
// as the same not-found. Only a gate failure is an error; callers surface it
// with writeGateError.
type visibleReader struct {
	store    store.Store
	resolver *visibility.Resolver
}

// newVisibleReader constructs a visibleReader over s.
func newVisibleReader(s store.Store) (visibleReader, error) {
	if s == nil {
		return visibleReader{}, errors.New("dataentry: newVisibleReader: store must be non-nil")
	}
	res, err := visibility.NewResolver(ctxRowGate{}, visibility.NopRedactor{}, s)
	if err != nil {
		return visibleReader{}, fmt.Errorf("dataentry: newVisibleReader: %w", err)
	}
	return visibleReader{store: s, resolver: res}, nil
}

// address reads the row a wire address names (`ID` or `ID@face`) in the
// request's world: a named face literally, a bare id through the world. An
// address the grammar rejects is a miss.
func (vr visibleReader) address(ctx context.Context, entityType, addr string) (*entitypkg.Entity, bool, error) {
	return rowOf(vr.resolver.Address(ctx, worldFromContext(ctx).visibility(), entityType, addr))
}

// addressRef is [visibleReader.address] for an address already parsed: a
// named face literally, a bare id through the request's world.
func (vr visibleReader) addressRef(
	ctx context.Context, entityType string, ref entitypkg.Ref,
) (*entitypkg.Entity, bool, error) {
	if ref.Face.IsDefault() {
		return vr.inWorld(ctx, entityType, ref.ID)
	}
	return vr.ref(ctx, entityType, ref)
}

// inWorld resolves a bare id to the face the request's world selects.
func (vr visibleReader) inWorld(ctx context.Context, entityType, id string) (*entitypkg.Entity, bool, error) {
	return vr.inWorldOf(ctx, worldFromContext(ctx).visibility(), entityType, id)
}

// inWorldOf is [visibleReader.inWorld] in an explicit world, for the view
// engine, whose callers pass the world rather than binding it to ctx.
func (vr visibleReader) inWorldOf(
	ctx context.Context, w visibility.World, entityType, id string,
) (*entitypkg.Entity, bool, error) {
	return rowOf(vr.resolver.InWorld(ctx, w, entityType, id))
}

// ref reads the row ref names, literally, in the request's world.
func (vr visibleReader) ref(
	ctx context.Context, entityType string, ref entitypkg.Ref,
) (*entitypkg.Entity, bool, error) {
	return vr.refIn(ctx, worldFromContext(ctx).visibility(), entityType, ref)
}

// refIn is [visibleReader.ref] in an explicit world; see
// [visibleReader.inWorldOf].
func (vr visibleReader) refIn(
	ctx context.Context, w visibility.World, entityType string, ref entitypkg.Ref,
) (*entitypkg.Entity, bool, error) {
	return rowOf(vr.resolver.Ref(ctx, w, entityType, ref))
}

// family reports the faces of id the principal may read. It answers the
// entity-level question "does a readable face of this id exist, of this
// type", and reads headers only.
func (vr visibleReader) family(ctx context.Context, entityType, id string) (visibility.Family, bool, error) {
	return vr.resolver.Family(ctx, entityType, id)
}

// untypedAddress is [visibleReader.address] for a request that carries no
// entity type (commands, detail actions). The type comes from the stored row
// ([visibleReader.storedType]), so the gates run on the real type, never on
// one the caller claims.
func (vr visibleReader) untypedAddress(ctx context.Context, addr string) (*entitypkg.Entity, bool, error) {
	ref, err := entitypkg.ParseRef(addr)
	if err != nil {
		return nil, false, nil //nolint:nilerr // a malformed address is a miss, not a fault
	}
	return vr.untypedRef(ctx, ref)
}

// untypedRef is [visibleReader.untypedAddress] for an address already parsed.
func (vr visibleReader) untypedRef(ctx context.Context, ref entitypkg.Ref) (*entitypkg.Entity, bool, error) {
	typ := vr.storedType(ctx, ref.ID)
	if typ == "" {
		return nil, false, nil
	}
	return vr.addressRef(ctx, typ, ref)
}

// readableType returns the stored type of id when the principal may read
// some face of it, and "" otherwise. It is [visibleReader.family] for a
// caller that holds only the id.
func (vr visibleReader) readableType(ctx context.Context, id string) (string, error) {
	typ := vr.storedType(ctx, id)
	if typ == "" {
		return "", nil
	}
	_, ok, err := vr.family(ctx, typ, id)
	if err != nil || !ok {
		return "", err
	}
	return typ, nil
}

// readableTypes is [visibleReader.readableType] for many ids at once; see
// [visibility.Resolver.ReadableTypes]. A failed read or gate is returned,
// never folded into a miss.
func (vr visibleReader) readableTypes(ctx context.Context, ids []string) (map[string]string, error) {
	return vr.resolver.ReadableTypes(ctx, ids)
}

// storedType is [storedTypeOf] over this reader's store.
func (vr visibleReader) storedType(ctx context.Context, id string) string {
	return storedTypeOf(ctx, vr.store, id)
}

// storedTypeOf returns the type of any stored face of id, or "" when no face
// exists or the read fails. It reads one content-free header and applies no
// gate, so its answer must never decide what is served. Callers pass it to a
// gated read, or use it for write authorization, which needs the real type
// whether or not the caller may read the row.
func storedTypeOf(ctx context.Context, st store.EntityLister, id string) string {
	typ, _ := storedFacesOf(ctx, st, id)
	return typ
}

// storedFacesOf is [storedTypeOf] plus every stored face of id. The same
// caveat applies: no gate runs, so the faces decide liveness, never what is
// served. A read error is logged and answered as "nothing stored", which is
// safe only where "nothing stored" leads to a refusal; a caller that grants
// more to a missing row than to a live one uses [loadStoredFaces].
func storedFacesOf(ctx context.Context, st store.EntityLister, id string) (string, []entitypkg.Face) {
	typ, faces, err := loadStoredFaces(ctx, st, id)
	if err != nil {
		slog.Warn("dataentry: reading an entity's stored faces failed", "id", id, "err", err)
		return "", nil
	}
	return typ, faces
}

// loadStoredFaces is [storedFacesOf] that returns the read error. The
// history surfaces need it: they open a deleted face's history on a global
// permission, so answering a failed read as "deleted" would fail open onto a
// live face the caller cannot see.
func loadStoredFaces(ctx context.Context, st store.EntityLister, id string) (string, []entitypkg.Face, error) {
	if id == "" {
		return "", nil, nil
	}
	var typ string
	var faces []entitypkg.Face
	q := store.EntityQuery{IDs: []string{id}, AllStates: true}
	for h, err := range store.ListEntityHeaders(ctx, st, q) {
		if err != nil {
			return "", nil, err
		}
		if h.ID == id {
			typ = h.Type
			faces = append(faces, h.Face)
		}
	}
	return typ, faces, nil
}

// rowOf drops the provenance a [visibility.Resolved] carries. The data-entry
// surfaces label provenance themselves (worldProvenance, addressedProvenance),
// which keeps their wire output unchanged.
func rowOf(res visibility.Resolved, ok bool, err error) (*entitypkg.Entity, bool, error) {
	if err != nil || !ok {
		return nil, false, err
	}
	return res.Entity, true, nil
}

// readAddressedOr404 reads the row the path segment addr names and writes
// the response itself when there is none: the uniform not-found for every
// miss, or the gate error. Every addressed route owes this read before it
// acts on a row, reads and writes alike. A face the principal may not read
// is then the same 404 as an absent one, never a 403 that confirms it exists.
func readAddressedOr404(
	w http.ResponseWriter, r *http.Request, vr visibleReader, entityType, addr string,
) (*entitypkg.Entity, bool) {
	e, ok, err := vr.address(r.Context(), entityType, addr)
	if err != nil {
		writeGateError(w, r, err)
		return nil, false
	}
	if !ok {
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
		return nil, false
	}
	return e, true
}

// familyReadableOr404 reports whether the principal may read some face of
// the entity id, and writes the uniform not-found when not. It is the check
// for a relation endpoint named by bare id (a body target, a head): such an
// id names an entity, never one face of it. The type comes from the stored
// row, so the gate runs on the real type.
func familyReadableOr404(w http.ResponseWriter, r *http.Request, vr visibleReader, id string) bool {
	typ, err := vr.readableType(r.Context(), id)
	if err != nil {
		writeGateError(w, r, err)
		return false
	}
	if typ == "" {
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
		return false
	}
	return true
}

// faceReadable reports whether this principal's read grants cover the given
// content state.
//
// Single-entity reads get this check from the resolver. It remains for the
// list-side paths, which gate a batch of rows they already hold (headers,
// neighbors, view collections), and for the history of a DELETED face, which
// has no row for the resolver to find (see resolveHistorySubject).
//
// An empty Faces slice means "every face"; see [acl.ReadQueryResult.Faces]
// for why a bare grant widens rather than narrows. It delegates to
// [visibility.FaceAllowed] through the same [ctxRowGate] the resolver uses, so
// this package holds no second copy of the rule.
func faceReadable(ctx context.Context, entityType string, face entitypkg.Face) bool {
	return visibility.FaceAllowed(ctx, ctxRowGate{}, entityType, face)
}

// filterVisible drops every candidate the principal cannot read, batching the
// gate probe by entity type — one PermitsReadMany per distinct type, turning a
// worst case of O(N) per-id probes into O(distinct-types) (RR-FRK1). Order is
// preserved and a fresh slice is returned (RR-I2SI). On a gate error for a
// type, that whole type is dropped fail-closed (RR-7TIU) — a read-ACL failure
// must never widen visibility — and logged loud so operators see the cause
// rather than a silently-empty include block.
//
// This is the extraction of the former App.filterVisibleIncludes; behavior is
// preserved, including the nil return for empty input.
// visibleHeaderIDs is the row-gate half of filterVisible over content-free
// headers: the set of candidate ids the principal may read, probed once per
// distinct type. No redaction is involved because only ids leave here — the
// caller uses the set to decide which neighbor ids may appear in a
// relations map, never to serve a property.
func (vr visibleReader) visibleHeaderIDs(ctx context.Context, candidates []store.EntityHeader) map[string]bool {
	out := make(map[string]bool, len(candidates))
	if len(candidates) == 0 {
		return out
	}
	gate := readGateFromContext(ctx)
	byType := make(map[string][]string)
	for _, c := range candidates {
		byType[c.Type] = append(byType[c.Type], c.ID)
	}
	allowed := make(map[string]bool, len(candidates))
	for typeName, ids := range byType {
		perm, err := gate.PermitsReadMany(ctx, typeName, ids)
		if err != nil {
			slog.Warn("dataentry: visibleReader.visibleHeaderIDs: PermitsReadMany failed; dropping type",
				"type", typeName, "candidates", len(ids), "err", err)
			continue
		}
		for id, ok := range perm {
			if ok {
				allowed[id] = true
			}
		}
	}
	// The face grant is the other half of filterVisible's gate (TKT-O7R2A1);
	// a header carries its Face, so applying it here keeps the two gates
	// from drifting when a caller one day passes AllStates or a World.
	for _, c := range candidates {
		if allowed[c.ID] && faceReadable(ctx, c.Type, c.Face) {
			out[c.ID] = true
		}
	}
	return out
}

func (vr visibleReader) filterVisible(ctx context.Context, candidates []*entitypkg.Entity) []*entitypkg.Entity {
	if len(candidates) == 0 {
		return nil
	}
	gate := readGateFromContext(ctx)

	byType := make(map[string][]*entitypkg.Entity)
	for _, c := range candidates {
		byType[c.Type] = append(byType[c.Type], c)
	}

	allowed := make(map[string]bool, len(candidates))
	for typeName, group := range byType {
		ids := make([]string, 0, len(group))
		for _, c := range group {
			ids = append(ids, c.ID)
		}
		perm, err := gate.PermitsReadMany(ctx, typeName, ids)
		if err != nil {
			slog.Warn("dataentry: visibleReader.filterVisible: PermitsReadMany failed; dropping type",
				"type", typeName,
				"candidates", len(ids),
				"err", err)
			continue
		}
		for id, ok := range perm {
			if ok {
				allowed[id] = true
			}
		}
	}

	// Preserve original candidate order; allocate a fresh slice. The face
	// half is owed HERE too (TKT-O7R2A1): the row gate is face-blind, and a
	// neighbor arrives as a resolved face — under a world, possibly a
	// within-chain fallback to a face a `type@face` grant withholds. Without
	// this a principal granted only `feature@published` saw a draft-only
	// neighbor's title through `?include=` while its own GET 404'd.
	out := make([]*entitypkg.Entity, 0, len(candidates))
	for _, c := range candidates {
		if allowed[c.ID] && faceReadable(ctx, c.Type, c.Face) {
			out = append(out, c)
		}
	}
	return out
}
