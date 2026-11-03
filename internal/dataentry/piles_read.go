package dataentry

import (
	"context"

	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/piles"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// pileResolver is the read seam the piles routes need: one batched,
// redacting header resolution in the request's world, the type of a
// readable family, and the face a bare-id add targets.
//
// Built from the same row gate and field redactor as every other read path
// (DEC-ZBI39P), so a pile never shows a row or a property value the item's
// own page would hide.
type pileResolver interface {
	ResolveHeadersErr(
		ctx context.Context, w visibility.World, refs []entityPkg.Ref,
	) (map[entityPkg.Ref]visibility.ResolvedHeader, error)
	ResolveIDsErr(ctx context.Context, w visibility.World, ids []string) (map[string]store.EntityHeader, error)
	ReadableTypes(ctx context.Context, ids []string) (map[string]string, error)
	WriteTarget(
		ctx context.Context, w visibility.World, entityType string, addr entityPkg.Address,
	) (entityPkg.Ref, bool, error)
}

// pileRow is one readable pile item: the item as the pile stores it and the
// redacted header of the face it serves in the request's world.
type pileRow struct {
	ref    entityPkg.Ref
	item   piles.Item
	header store.EntityHeader
}

// readableItems returns the items of p the principal may read in the
// request's world, newest first. A named face is served literally; a bare
// item takes the face the world serves it at.
//
// A fault is returned, never answered as an empty pile: an empty answer
// would look like every item had been hidden.
func (h *pilesHandler) readableItems(ctx context.Context, p piles.Pile) ([]pileRow, error) {
	byPile, err := h.readablePiles(ctx, []piles.Pile{p})
	if err != nil {
		return nil, err
	}
	return byPile[p.ID], nil
}

// readablePiles is [pilesHandler.readableItems] for several piles at once.
// It resolves the union of their items in ONE batch, so listing every pile
// costs the same reads as showing one.
func (h *pilesHandler) readablePiles(ctx context.Context, ps []piles.Pile) (map[string][]pileRow, error) {
	var refs []entityPkg.Ref
	for _, p := range ps {
		for _, it := range p.Items {
			refs = append(refs, it.Ref)
		}
	}
	out := make(map[string][]pileRow, len(ps))
	if len(refs) == 0 {
		return out, nil
	}
	resolved, err := h.resolver.ResolveHeadersErr(ctx, worldFromContext(ctx).visibility(), refs)
	if err != nil {
		return nil, err
	}
	for _, p := range ps {
		rows := make([]pileRow, 0, len(p.Items))
		for _, it := range p.Items {
			res, ok := resolved[it.Ref]
			if !ok || !res.Served() {
				continue
			}
			rows = append(rows, pileRow{ref: it.Ref, item: it, header: res.Header})
		}
		out[p.ID] = rows
	}
	return out, nil
}

// pileEntities converts readable rows to body-less entities for the scope
// and export paths. Each carries the face it was served at, so its Ref is
// the row the reader sees.
func pileEntities(rows []pileRow) []*entityPkg.Entity {
	out := make([]*entityPkg.Entity, len(rows))
	for i, row := range rows {
		out[i] = headerEntity(row.header)
	}
	return out
}

// scopeEntities is the ordered set of a `source: pile` scope: the pile's
// readable items in the request's world. Without the service every pile is
// missing.
func (h *pilesHandler) scopeEntities(ctx context.Context, id string) ([]*entityPkg.Entity, error) {
	if h.svc == nil {
		return nil, piles.ErrNotFound
	}
	p, err := h.svc.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	rows, err := h.readableItems(ctx, p)
	if err != nil {
		return nil, err
	}
	return pileEntities(rows), nil
}

// actorTitles names the actors of rows. Under person mapping an actor is a
// person id: it resolves in one batch through the same gate as the items,
// and a person the owner cannot read is left unnamed, so the pile never
// shows more than the person's own page would. Otherwise an actor is a
// login and is shown as it stands.
func (h *pilesHandler) actorTitles(ctx context.Context, rows []pileRow) (map[string]string, error) {
	out := map[string]string{}
	if !h.svc.ActorsArePeople() {
		for _, row := range rows {
			if row.item.Actor != "" {
				out[row.item.Actor] = row.item.Actor
			}
		}
		return out, nil
	}
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.item.Actor != "" {
			ids = append(ids, row.item.Actor)
		}
	}
	if len(ids) == 0 {
		return out, nil
	}
	// An actor is a stored person id with no face; the world picks the face
	// the owner may read, as it does for a bare item.
	served, err := h.resolver.ResolveIDsErr(ctx, worldFromContext(ctx).visibility(), ids)
	if err != nil {
		return nil, err
	}
	meta := h.meta()
	for id, header := range served {
		out[id] = safeDisplayTitle(meta, headerEntity(header))
	}
	return out, nil
}
