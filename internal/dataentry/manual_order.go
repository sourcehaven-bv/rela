package dataentry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/listorder"
)

// Manual list order (TKT-AVNEX5T).
//
// A list with `order: manual` shows each user's own order: the rows they
// placed first, in their order, then the rest in the list sort. The order
// is stored per user in internal/listorder; membership stays computed by
// the list. Positions only permute the rows the pipeline already gated, so
// a hidden row takes no slot.

const (
	// manualOrderParam asks a list read for the caller's stored order. The
	// SPA sends it while the reader has not sorted; a grouped list always
	// sends its group sort, so the sort parameter cannot be the signal.
	manualOrderParam = "manual_order"
	// listOrderPath moves one row of a manual list.
	listOrderPath = "/api/v1/_list_order"
	// maxListOrderBody bounds the move body: three ids and their keys.
	maxListOrderBody = 4096
)

// listOrderStore is what the list pipeline and the move route need of the
// order service.
type listOrderStore interface {
	Order(ctx context.Context, list string) ([]string, error)
	Move(ctx context.Context, list string, full []string, id, before, after string) error
}

// SetListOrder wires the manual-order service. Without it a manual list
// shows its sort and the move route answers 404.
func SetListOrder(a *App, svc *listorder.Service) error {
	if svc == nil {
		return errors.New("dataentry: SetListOrder requires a list order service")
	}
	a.listOrder = svc
	return nil
}

// manualListFor returns the manual list a read applies the stored order of,
// or nil. The read must name the list in list_id and ask with
// manual_order=1, and the list must declare `order: manual`.
func manualListFor(s *Schema, query map[string][]string) (string, *dataentryconfig.List) {
	if queryGet(query, manualOrderParam) != "1" || s == nil || s.Cfg == nil {
		return "", nil
	}
	id := queryGet(query, listIDParam)
	l, ok := s.Cfg.Lists[id]
	if !ok || l.Order != dataentryconfig.ListOrderManual {
		return "", nil
	}
	return id, &l
}

// applyManualOrder puts rows in the caller's stored order of list. On a
// grouped list the order applies inside each run of equal group values, so
// the sections keep the order the sort gave them. A failed read keeps the
// sort: an order is presentation, not data. Call it after every filter and
// before paging, so it permutes only the gated rows the page describes.
func applyManualOrder(
	ctx context.Context, store listOrderStore, listID string, l *dataentryconfig.List, rows []*entityPkg.Entity,
) []*entityPkg.Entity {
	ids, err := store.Order(ctx, listID)
	if err != nil {
		if !errors.Is(err, listorder.ErrNoOwner) {
			slog.WarnContext(ctx, "manual list order: read failed", "list", listID, "err", err)
		}
		return rows
	}
	if len(ids) == 0 {
		return rows
	}
	group := ""
	if l.GroupBy != nil {
		group = l.GroupBy.Property
	}
	out := make([]*entityPkg.Entity, 0, len(rows))
	for start := 0; start < len(rows); {
		end := start + 1
		for end < len(rows) && sameGroup(rows[start], rows[end], group) {
			end++
		}
		out = append(out, permuted(rows[start:end], ids)...)
		start = end
	}
	return out
}

func sameGroup(a, b *entityPkg.Entity, prop string) bool {
	if prop == "" {
		return true
	}
	return fmt.Sprint(a.Properties[prop]) == fmt.Sprint(b.Properties[prop])
}

func permuted(rows []*entityPkg.Entity, ids []string) []*entityPkg.Entity {
	keys := make([]string, len(rows))
	for i, e := range rows {
		keys[i] = e.ID
	}
	out := make([]*entityPkg.Entity, len(rows))
	for i, j := range listorder.Apply(keys, ids) {
		out[i] = rows[j]
	}
	return out
}

// listOrderRequest is the body of a move.
type listOrderRequest struct {
	ID     string `json:"id"`
	Before string `json:"before,omitempty"`
	After  string `json:"after,omitempty"`
}

// handleV1ListOrder serves POST /api/v1/_list_order?list_id=…: it moves one
// row of a manual list next to a neighbor in the caller's own order. The
// query carries the same parameters as the list read on screen, so the
// move is judged against exactly the rows the caller sees, always in the
// stored order. A row or
// neighbor that is not among them is the uniform 404, whether it is
// hidden, missing, or in another list.
//
// A package function rather than an App method: App is at its plimsoll
// method line.
//
//archplan:auth=required
func handleV1ListOrder(a *App, w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodPost) {
		return
	}
	query, s := r.URL.Query(), a.State()
	query.Set(manualOrderParam, "1")
	listID, l := manualListFor(s, query)
	if a.listOrder == nil || l == nil {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Not found", "no manual list named in list_id")
		return
	}
	var req listOrderRequest
	body, err := io.ReadAll(io.LimitReader(r.Body, maxListOrderBody+1))
	if err != nil || len(body) > maxListOrderBody {
		writeV1Error(w, r, http.StatusRequestEntityTooLarge, "body_too_large", "Request body too large", "")
		return
	}
	if jsonErr := json.Unmarshal(body, &req); jsonErr != nil {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_json", "Invalid JSON body", jsonErr.Error())
		return
	}
	rows, _, _, err := a.lists.listPage(r.Context(), l.EntityType, query, 1, math.MaxInt32)
	if err != nil {
		writeListPipelineError(w, r, err)
		return
	}
	full := make([]string, len(rows))
	for i, e := range rows {
		full[i] = e.ID
	}
	switch err := a.listOrder.Move(r.Context(), listID, full, req.ID, req.Before, req.After); {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, listorder.ErrBadMove):
		writeV1Error(w, r, http.StatusBadRequest, "bad_request", "Invalid move", "name exactly one other row as before or after")
	case errors.Is(err, listorder.ErrNotFound):
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
	case errors.Is(err, listorder.ErrNoOwner):
		writeV1Error(w, r, http.StatusForbidden, "forbidden", "Forbidden", "a manual order needs a signed-in user")
	default:
		slog.ErrorContext(r.Context(), "manual list order: move failed", "list", listID, "err", err)
		writeV1Error(w, r, http.StatusInternalServerError, "internal_error", "Internal error", "")
	}
}

// manualOrderApplies reports whether a list read is shown in the caller's
// manual order: it names a manual list, asks for the order, the order
// service is wired, and the caller is a user who can hold an order. The
// SPA offers dragging only then.
func manualOrderApplies(ctx context.Context, p *listPipeline, s *Schema, query map[string][]string) bool {
	_, l := manualListFor(s, query)
	return l != nil && p.listOrder() != nil && listorder.HasOwner(ctx)
}
