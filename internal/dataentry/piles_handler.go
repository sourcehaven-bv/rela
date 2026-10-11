package dataentry

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/piles"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

const (
	pilesPath = "/api/v1/_piles"
	// pilesBodyLimit caps a piles request body. 500 addresses of a long id
	// and face fit with room to spare.
	pilesBodyLimit = 64 << 10
)

// errPilesBodyTooLarge reports a body over [pilesBodyLimit].
var errPilesBodyTooLarge = errors.New("request body too large")

// handleV1Piles serves every route under /api/v1/_piles:
//
//	GET    /_piles                          list (counts of readable items)
//	POST   /_piles                          create
//	GET    /_piles/{id}                     one pile with its readable items
//	PATCH  /_piles/{id}                     rename / re-icon
//	DELETE /_piles/{id}                     delete
//	POST   /_piles/{id}/items               add addresses
//	POST   /_piles/{id}/items/_remove       remove addresses (always 204)
//	GET    /_piles/{id}/_export?transform=  download
//
// A pile is private to its owner. Every per-pile route reaches the service
// with the acting user as owner, and the store answers another user's pile
// exactly like a missing one, so a foreign id is the same 404 as an unknown
// one. Items are read through the visibility resolver in the request's
// world; a hidden item is absent from every answer, counts included.
//
// Pile names are user content: they are never logged.
func (h *pilesHandler) handleV1Piles(w http.ResponseWriter, r *http.Request) {
	if h.svc == nil {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Not found", "")
		return
	}
	// Every route needs an owner. Checking it first gives a principal with
	// no identity one answer, whatever path it asks for.
	if _, err := h.svc.Owner(r.Context()); err != nil {
		writePilesError(w, r, err)
		return
	}
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, pilesPath), "/")
	var parts []string
	if rest != "" {
		parts = strings.Split(rest, "/")
	}
	switch {
	case len(parts) == 0:
		h.routeCollection(w, r)
	case len(parts) == 1:
		h.routePile(w, r, parts[0])
	case len(parts) == 2 && parts[1] == "items":
		if !allowMethod(w, r, http.MethodPost) {
			return
		}
		h.addItems(w, r, parts[0])
	case len(parts) == 3 && parts[1] == "items" && parts[2] == "_remove":
		if !allowMethod(w, r, http.MethodPost) {
			return
		}
		h.removeItems(w, r, parts[0])
	case len(parts) == 2 && parts[1] == "_export":
		if !allowMethod(w, r, http.MethodGet) {
			return
		}
		h.exportPile(w, r, parts[0])
	default:
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Not found", "")
	}
}

// allowMethod answers 405 unless r uses method.
func allowMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method == method {
		return true
	}
	w.Header().Set("Allow", method)
	writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
	return false
}

func (h *pilesHandler) routeCollection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listPiles(w, r)
	case http.MethodPost:
		h.createPile(w, r)
	default:
		w.Header().Set("Allow", "GET, POST")
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
	}
}

func (h *pilesHandler) routePile(w http.ResponseWriter, r *http.Request, id string) {
	switch r.Method {
	case http.MethodGet:
		p, err := h.svc.Get(r.Context(), id)
		if err != nil {
			writePilesError(w, r, err)
			return
		}
		h.writePile(w, r, http.StatusOK, p)
	case http.MethodPatch:
		var req v1.PileUpdateRequest
		if !decodePilesBody(w, r, &req) {
			return
		}
		p, err := h.svc.Update(r.Context(), id, req.Name, req.Icon)
		if err != nil {
			writePilesError(w, r, err)
			return
		}
		h.writePile(w, r, http.StatusOK, p)
	case http.MethodDelete:
		if err := h.svc.Delete(r.Context(), id); err != nil {
			writePilesError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		w.Header().Set("Allow", "GET, PATCH, DELETE")
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
	}
}

// listPiles answers GET /_piles. Every pile's count is taken AFTER the
// read gate, over one batch holding the union of all the owner's items.
func (h *pilesHandler) listPiles(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ps, err := h.svc.List(ctx)
	if err != nil {
		writePilesError(w, r, err)
		return
	}
	readable, err := h.readablePiles(ctx, ps)
	if err != nil {
		writeGateError(w, r, err)
		return
	}
	out := v1.PileList{Piles: make([]v1.PileSummary, len(ps)), Icons: piles.Icons}
	for i, p := range ps {
		out.Piles[i] = pileSummary(p, len(readable[p.ID]))
	}
	writeV1JSON(w, http.StatusOK, out)
}

func (h *pilesHandler) createPile(w http.ResponseWriter, r *http.Request) {
	var req v1.PileCreateRequest
	if !decodePilesBody(w, r, &req) {
		return
	}
	refs, ok := h.addRefs(w, r, req.Items)
	if !ok {
		return
	}
	p, err := h.svc.Create(r.Context(), piles.CreateRequest{Name: req.Name, Icon: req.Icon, Refs: refs})
	if err != nil {
		writePilesError(w, r, err)
		return
	}
	h.writePile(w, r, http.StatusCreated, p)
}

func (h *pilesHandler) addItems(w http.ResponseWriter, r *http.Request, id string) {
	var req v1.PileItemsRequest
	if !decodePilesBody(w, r, &req) {
		return
	}
	// The pile is looked up before any address is resolved, so a foreign
	// pile answers its 404 without first telling the caller anything about
	// the addresses.
	if _, err := h.svc.Get(r.Context(), id); err != nil {
		writePilesError(w, r, err)
		return
	}
	refs, ok := h.addRefs(w, r, req.Items)
	if !ok {
		return
	}
	added, err := h.svc.Add(r.Context(), id, refs)
	if err != nil {
		writePilesError(w, r, err)
		return
	}
	writeV1JSON(w, http.StatusOK, v1.PileAdded{Added: added})
}

// removeItems answers POST /_piles/{id}/items/_remove with 204 for any
// addresses: hidden, deleted, never added and malformed alike. Nothing is
// resolved, so the answer cannot tell those apart.
func (h *pilesHandler) removeItems(w http.ResponseWriter, r *http.Request, id string) {
	var req v1.PileItemsRequest
	if !decodePilesBody(w, r, &req) {
		return
	}
	refs := make([]entityPkg.Ref, 0, len(req.Items))
	for _, addr := range req.Items {
		if ref, err := entityPkg.ParseRef(strings.TrimSpace(addr)); err == nil {
			refs = append(refs, ref)
		}
	}
	if err := h.svc.Remove(r.Context(), id, refs); err != nil {
		writePilesError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// addRefs resolves the addresses of an add to the refs the pile stores, in
// request order. It writes the response and returns ok=false on any miss.
//
// Every address resolves in one batch in the request's world. A named face
// must be served as named. A bare id is accepted as it stands only for a
// faceless type, whose only face is the implicit one. Any other bare id of
// a readable family goes through [visibility.Resolver.WriteTarget], which
// refuses to pick a face: the answer is a 409 naming the readable faces.
//
// One unreadable or missing address fails the whole request with a 404
// that does not say which.
func (h *pilesHandler) addRefs(w http.ResponseWriter, r *http.Request, addrs []string) ([]entityPkg.Ref, bool) {
	if len(addrs) > piles.MaxItems {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_request",
			"Too many items", "a request may name at most 500 items")
		return nil, false
	}
	refs := make([]entityPkg.Ref, len(addrs))
	for i, addr := range addrs {
		ref, err := entityPkg.ParseRef(strings.TrimSpace(addr))
		if err != nil || ref.IsZero() {
			writeV1Error(w, r, http.StatusBadRequest, "invalid_request", "Invalid item address", "")
			return nil, false
		}
		refs[i] = ref
	}
	if len(refs) == 0 {
		return refs, true
	}
	ctx := r.Context()
	world := worldFromContext(ctx).visibility()
	resolved, err := h.resolver.ResolveHeadersErr(ctx, world, refs)
	if err != nil {
		writeGateError(w, r, err)
		return nil, false
	}
	var pending []string // bare ids of a readable family that need a face
	for _, ref := range refs {
		res, found := resolved[ref]
		switch {
		case ref.Face.IsImplicit() && found && res.Served() && res.Header.Face.IsImplicit():
			// A faceless type: the bare id is the ref.
		case !ref.Face.IsImplicit() && found && res.Served():
		case ref.Face.IsImplicit() && found && res.Family:
			pending = append(pending, ref.ID)
		default:
			writeItemNotFound(w, r)
			return nil, false
		}
	}
	if len(pending) > 0 {
		if !h.namedFaces(w, r, world, pending) {
			return nil, false
		}
	}
	return refs, true
}

// namedFaces asks WriteTarget about each bare id the batch could not settle.
// A faced type has no implicit face, so WriteTarget refuses to pick one and
// the answer is a 409 naming the faces. A faceless type it admits is kept
// as the bare id; anything else is the uniform 404. It returns false once it
// has written the response.
func (h *pilesHandler) namedFaces(w http.ResponseWriter, r *http.Request, world visibility.World, ids []string) bool {
	ctx := r.Context()
	types, err := h.resolver.ReadableTypes(ctx, ids)
	if err != nil {
		writeGateError(w, r, err)
		return false
	}
	for _, id := range ids {
		typ, ok := types[id]
		if !ok {
			writeItemNotFound(w, r)
			return false
		}
		_, ok, err := h.resolver.WriteTarget(ctx, world, typ, entityPkg.BareAddress(id))
		var amb *visibility.AmbiguousAddressError
		switch {
		case errors.As(err, &amb):
			writeAmbiguousAddress(w, r, amb)
			return false
		case err != nil:
			writeGateError(w, r, err)
			return false
		case !ok:
			writeItemNotFound(w, r)
			return false
		}
	}
	return true
}

// writeItemNotFound is the one answer for an add naming an address the
// principal cannot read or that does not exist. It never names the address.
func writeItemNotFound(w http.ResponseWriter, r *http.Request) {
	writeV1Error(w, r, http.StatusNotFound, "item_not_found", "Item not found",
		"an item does not exist or you cannot read it")
}

// writeAmbiguousAddress answers a bare id of a faced type. Same body as
// writeFaceRequired, under the piles contract's 409: the faces are the ones
// the principal may read, so naming them discloses nothing new.
func writeAmbiguousAddress(w http.ResponseWriter, r *http.Request, amb *visibility.AmbiguousAddressError) {
	faces := make([]string, len(amb.Faces))
	for i, f := range amb.Faces {
		faces[i] = entityPkg.FormatStateRef(amb.ID, f)
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(http.StatusConflict)
	_ = json.NewEncoder(w).Encode(v1.Error{
		Type:     "https://rela.dev/errors/ambiguous_address",
		Title:    "Address one face",
		Status:   http.StatusConflict,
		Detail:   amb.Error(),
		Instance: r.URL.Path,
		Faces:    faces,
	})
}

// writePile answers one pile with its readable items.
func (h *pilesHandler) writePile(w http.ResponseWriter, r *http.Request, status int, p piles.Pile) {
	rows, err := h.readableItems(r.Context(), p)
	if err != nil {
		writeGateError(w, r, err)
		return
	}
	meta := h.meta()
	out := v1.Pile{PileSummary: pileSummary(p, len(rows)), Items: make([]v1.PileItem, len(rows))}
	for i, row := range rows {
		out.Items[i] = v1.PileItem{
			ID:      row.ref.ID,
			Face:    string(row.ref.Face),
			Address: row.ref.String(),
			Type:    row.header.Type,
			Title:   safeDisplayTitle(meta, headerEntity(row.header)),
		}
	}
	writeV1JSON(w, status, out)
}

// pileSummary is the wire summary of p with count readable items.
func pileSummary(p piles.Pile, count int) v1.PileSummary {
	return v1.PileSummary{
		ID:      p.ID,
		Name:    p.Name,
		Icon:    p.Icon,
		Count:   count,
		Created: p.Created.UTC().Format(time.RFC3339),
		Updated: p.Updated.UTC().Format(time.RFC3339),
	}
}

// decodePilesBody reads a capped JSON body into dst, answering 413 past the
// cap and 400 for anything that is not JSON.
func decodePilesBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	data, err := io.ReadAll(io.LimitReader(r.Body, pilesBodyLimit+1))
	if err == nil && len(data) > pilesBodyLimit {
		err = errPilesBodyTooLarge
	}
	switch {
	case errors.Is(err, errPilesBodyTooLarge):
		writeV1Error(w, r, http.StatusRequestEntityTooLarge, "body_too_large", "Request body too large", "")
		return false
	case err != nil:
		writeV1Error(w, r, http.StatusBadRequest, "invalid_request", "Could not read the request body", "")
		return false
	}
	if err := json.Unmarshal(data, dst); err != nil {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_request", "Invalid JSON", "")
		return false
	}
	return true
}

// writePilesError maps a piles service error to its response. The service
// errors carry no pile name, and neither does the log line.
func writePilesError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, context.Canceled):
		return
	case errors.Is(err, piles.ErrNoOwner), errors.Is(err, piles.ErrUnknownOwner):
		writeV1Error(w, r, http.StatusForbidden, "no_owner", "Piles need a signed-in user", "")
	case errors.Is(err, piles.ErrNotFound):
		writeV1Error(w, r, http.StatusNotFound, "pile_not_found", "Pile not found", "")
	case errors.Is(err, piles.ErrLimit):
		writeV1Error(w, r, http.StatusConflict, "pile_limit", "Too many piles",
			"a user may keep at most 50 piles")
	case errors.Is(err, piles.ErrNameTaken):
		writeV1Error(w, r, http.StatusConflict, "pile_name_taken", "A pile with this name exists", "")
	case errors.Is(err, piles.ErrInvalid):
		writeV1Error(w, r, http.StatusBadRequest, "invalid_request", "Invalid pile request",
			strings.TrimPrefix(err.Error(), piles.ErrInvalid.Error()+": "))
	default:
		slog.Warn("dataentry: piles request failed", "err", err, "method", r.Method)
		writeV1Error(w, r, http.StatusInternalServerError, "piles_failed", "Piles request failed",
			"check server logs")
	}
}
