package dataentry

import (
	"context"
	"strconv"

	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Content-free rows (TKT-1U8XYN).
//
// The list, search and scope-navigation pipelines filter, sort and paginate
// over a whole type before they know which rows they will serve, and the
// rows they do serve render titles and properties, never bodies. So those
// pipelines read HEADERS and carry them as content-free entities; a page's
// bodies are loaded afterwards, and only when the caller asked for them
// with `include_content=true`.
//
// A content-free entity is an *entity.Entity whose Content is empty because
// it was never read — indistinguishable, by type, from an entity with an
// empty body. That is the trade TKT-1ESTYJ warned about, and it is
// contained rather than avoided: the only code that receives these rows is
// the list pipeline (which reads properties) and the serializer (whose
// `content` field is omitempty, so an unloaded body is simply absent on
// the wire — the same shape as an entity without one). Nothing here may
// hand such a row to a body-dependent consumer (ETag computation, mention
// scanning, export); those keep reading whole entities.

// includeContentParam is the query parameter that asks a collection
// response to carry each row's body. Off by default: no SPA collection
// surface renders a body, and a 100-row kanban page shipped ~500 KB of
// markdown nobody read.
const includeContentParam = "include_content"

// wantContent reports whether the request opted into row bodies. Anything
// but a parseable true is false.
func wantContent(query map[string][]string) bool {
	v, _ := strconv.ParseBool(queryGet(query, includeContentParam))
	return v
}

// headerEntity is the content-free entity a header projects to. See the
// file comment for where these may and may not travel.
func headerEntity(h store.EntityHeader) *entityPkg.Entity {
	return &entityPkg.Entity{
		ID:           h.ID,
		Type:         h.Type,
		Face:         h.Face,
		Properties:   h.Properties,
		UpdatedAt:    h.UpdatedAt,
		Redacted:     h.Redacted,
		Inaccessible: h.Inaccessible,
	}
}

// rowKey addresses one (id, face) row.
type rowKey struct {
	id   string
	face entityPkg.Face
}

// loadRows reads the rows keys name, grouped by face: default-face rows in
// one IDs query, each other face in one AllStates query narrowed to that
// face on the way out. Returns only rows that exist.
func loadRows(ctx context.Context, st store.Store, keys []rowKey) (map[rowKey]*entityPkg.Entity, error) {
	byFace := make(map[entityPkg.Face][]string)
	seen := make(map[rowKey]struct{}, len(keys))
	for _, k := range keys {
		if _, dup := seen[k]; dup {
			continue
		}
		seen[k] = struct{}{}
		byFace[k.face] = append(byFace[k.face], k.id)
	}
	out := make(map[rowKey]*entityPkg.Entity, len(seen))
	for face, ids := range byFace {
		q := store.EntityQuery{IDs: ids, AllStates: !face.IsDefault()}
		for e, err := range st.ListEntities(ctx, q) {
			if err != nil {
				return nil, err
			}
			if e.Face != face {
				continue // AllStates over-returns the family's other faces
			}
			out[rowKey{e.ID, e.Face}] = e
		}
	}
	return out, nil
}

// loadRowContent replaces each content-free row with its whole entity, in
// place, in one read per distinct face. A row the store no longer has keeps
// its content-free form rather than vanishing from a page that was already
// counted. Redaction survives: the whole entity is re-projected onto the
// row's redacted property set, so a hidden value never rides in on the body.
func loadRowContent(ctx context.Context, st store.Store, rows []*entityPkg.Entity) error {
	if len(rows) == 0 {
		return nil
	}
	keys := make([]rowKey, 0, len(rows))
	for _, r := range rows {
		keys = append(keys, rowKey{r.ID, r.Face})
	}
	loaded, err := loadRows(ctx, st, keys)
	if err != nil {
		return err
	}
	for _, r := range rows {
		if full, ok := loaded[rowKey{r.ID, r.Face}]; ok {
			r.Content = full.Content
		}
	}
	return nil
}
