package appbuild

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/realtime"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// triggerEventBuffer is the store subscription buffer of the trigger
// engine. The watcher drops events past it; the next reconcile catches
// what was dropped.
const triggerEventBuffer = 1024

// triggerScanPage is how many rows the trigger engine's full scan hands
// over at a time.
const triggerScanPage = 500

// entitySource is the [realtime.EntitySource] over the raw store. Like
// [anchorExistence] it reads headers with no ACL: the engine acts for the
// system, a rule may read only properties no `visible:` grant restricts,
// and a post carries operator literals, never a value it read. A family
// rule reads the default world's face, as a family channel's readers see.
type entitySource struct {
	st    store.Store
	world store.WorldScope
}

func (s entitySource) query(entityType string, m realtime.FaceMode) store.EntityQuery {
	q := store.EntityQuery{Type: entityType, Faces: store.InWorld(s.world)}
	if m == realtime.FaceEach {
		q.Faces = store.AllFaces()
	}
	return q
}

// Rows implements [realtime.EntitySource] with one header read.
func (s entitySource) Rows(
	ctx context.Context, entityType string, ids []string, m realtime.FaceMode,
) ([]realtime.EntityRow, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	q := s.query(entityType, m)
	q.IDs = ids
	var out []realtime.EntityRow
	for h, err := range store.ListEntityHeaders(ctx, s.st, q) {
		if err != nil {
			return nil, err
		}
		out = append(out, rowOf(h))
	}
	return out, nil
}

// Scan implements [realtime.EntitySource] with one streamed header read.
func (s entitySource) Scan(
	ctx context.Context, entityType string, m realtime.FaceMode, page func([]realtime.EntityRow) error,
) error {
	buf := make([]realtime.EntityRow, 0, triggerScanPage)
	for h, err := range store.ListEntityHeaders(ctx, s.st, s.query(entityType, m)) {
		if err != nil {
			return err
		}
		buf = append(buf, rowOf(h))
		if len(buf) == triggerScanPage {
			if err := page(buf); err != nil {
				return err
			}
			buf = buf[:0]
		}
	}
	if len(buf) == 0 {
		return nil
	}
	return page(buf)
}

func rowOf(h store.EntityHeader) realtime.EntityRow {
	row := realtime.EntityRow{ID: h.ID, Face: h.Face, Props: h.Properties}
	for _, f := range h.Inaccessible {
		row.Inaccessible = append(row.Inaccessible, f.Name)
	}
	return row
}

// changeOf is the trigger engine's view of a store event, or false for a
// relation event: no rule reads relations.
func changeOf(ev store.Event) (realtime.Change, bool) {
	switch ev.Op {
	case store.EventEntityCreated, store.EventEntityUpdated, store.EventEntityDeleted:
		return realtime.Change{EntityType: ev.EntityType, ID: ev.EntityID}, ev.EntityID != ""
	default:
		return realtime.Change{}, false
	}
}
