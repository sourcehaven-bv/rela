package appbuild

import (
	"context"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/realtime"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// triggerEventBuffer is the store subscription buffer of the trigger
// engine. The watcher drops events past it; the next reconcile catches
// what was dropped.
const triggerEventBuffer = 1024

// triggerScanPage is how many rows the trigger engine's full scan hands
// over at a time.
const triggerScanPage = 500

// entitySource is the [realtime.EntitySource] over the store. It reads
// headers through the allow-all capability: the engine acts for the
// system, a rule may read only properties no `visible:` grant restricts,
// and a post carries operator literals, never a value it read. A family
// rule reads the default world's face, as a family channel's readers see.
type entitySource struct {
	st    store.Store
	gate  headerFilter
	world store.WorldScope
}

// headerFilter is the read gate entitySource puts every header through.
type headerFilter interface {
	FilterHeaders(ctx context.Context, candidates []store.EntityHeader) []store.EntityHeader
}

// newEntitySource returns the source over st, gated by a
// [visibility.AllowAllReader]: the capability is handed out here, at the
// wiring site, while the worker keeps its system:realtime principal.
func newEntitySource(st store.Store, world store.WorldScope) (entitySource, error) {
	gate, err := visibility.NewAllowAllReader(st)
	if err != nil {
		return entitySource{}, fmt.Errorf("realtime triggers: %w", err)
	}
	return entitySource{st: st, gate: gate, world: world}, nil
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
	var hs []store.EntityHeader
	for h, err := range store.ListEntityHeaders(ctx, s.st, q) {
		if err != nil {
			return nil, err
		}
		hs = append(hs, h)
	}
	hs = s.gate.FilterHeaders(ctx, hs)
	out := make([]realtime.EntityRow, 0, len(hs))
	for _, h := range hs {
		out = append(out, rowOf(h))
	}
	return out, nil
}

// Scan implements [realtime.EntitySource] with one streamed header read.
func (s entitySource) Scan(
	ctx context.Context, entityType string, m realtime.FaceMode, page func([]realtime.EntityRow) error,
) error {
	buf := make([]store.EntityHeader, 0, triggerScanPage)
	flush := func() error {
		hs := s.gate.FilterHeaders(ctx, buf)
		buf = buf[:0]
		if len(hs) == 0 {
			return nil
		}
		rows := make([]realtime.EntityRow, 0, len(hs))
		for _, h := range hs {
			rows = append(rows, rowOf(h))
		}
		return page(rows)
	}
	for h, err := range store.ListEntityHeaders(ctx, s.st, s.query(entityType, m)) {
		if err != nil {
			return err
		}
		buf = append(buf, h)
		if len(buf) == triggerScanPage {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	return flush()
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
