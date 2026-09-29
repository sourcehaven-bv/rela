package appbuild

import (
	"context"
	"iter"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/worldreader"
)

// WorldBound returns b with its entity reads and searches resolved through
// the world that source returns (BUG-6XTX0G).
//
// A consumer of [Services.GatedReads] addresses entities by id and never
// names a world, so without this it reads the default world, where a faced
// type has no row at all. Binding the world here, once for the whole bundle,
// is what keeps the MCP tools and the Lua reads from each having to remember
// it.
//
// It wraps the entity reader, the searcher and the Lua read handles. The
// tracer and the validator are unchanged, as are relation reads and counts,
// which are keyed by the bare id and are not face-resolved.
//
// A package-level function rather than a Services method for the same reason
// [CompiledWorlds] is one: Services sits at its plimsoll exported-method cap.
//
// Nil: source is rejected, because a bundle with no world would silently read
// the default world.
func WorldBound(b GatedReadBundle, source worldreader.Source) (GatedReadBundle, error) {
	rows, err := worldreader.NewBoundReader(b.Reader, source)
	if err != nil {
		return GatedReadBundle{}, err
	}
	b.Reader = worldGraphReader{GatedGraphReader: b.Reader, rows: rows}
	if b.LuaReads.VisibleReader != nil {
		luaRows, err := worldreader.NewBoundReader(b.LuaReads.VisibleReader, source)
		if err != nil {
			return GatedReadBundle{}, err
		}
		b.LuaReads.VisibleReader = worldLuaReader{EntityReader: b.LuaReads.VisibleReader, rows: luaRows}
	}
	b.Searcher = bindSearcher(b.Searcher, source)
	b.LuaReads.Searcher = bindSearcher(b.LuaReads.Searcher, source)
	return b, nil
}

// worldGraphReader serves entity reads from the world-bound reader and
// everything else from the gated reader it wraps.
type worldGraphReader struct {
	GatedGraphReader
	rows *worldreader.BoundReader
}

func (w worldGraphReader) GetEntity(ctx context.Context, id string) (*entity.Entity, error) {
	return w.rows.GetEntity(ctx, id)
}

func (w worldGraphReader) ListEntities(ctx context.Context, q store.EntityQuery) iter.Seq2[*entity.Entity, error] {
	return w.rows.ListEntities(ctx, q)
}

// worldLuaReader is [worldGraphReader] for the Lua read surface.
type worldLuaReader struct {
	lua.EntityReader
	rows *worldreader.BoundReader
}

func (w worldLuaReader) GetEntity(ctx context.Context, id string) (*entity.Entity, error) {
	return w.rows.GetEntity(ctx, id)
}

func (w worldLuaReader) ListEntities(ctx context.Context, q store.EntityQuery) iter.Seq2[*entity.Entity, error] {
	return w.rows.ListEntities(ctx, q)
}

// worldSearcher stamps the bound world onto a search that names none, so a
// hit is the face the world serves.
type worldSearcher struct {
	inner  search.Searcher
	source worldreader.Source
}

// bindSearcher wraps inner, or returns nil when there is no searcher to bind.
func bindSearcher(inner search.Searcher, source worldreader.Source) search.Searcher {
	if inner == nil {
		return nil
	}
	return worldSearcher{inner: inner, source: source}
}

func (w worldSearcher) Search(ctx context.Context, q search.Query) iter.Seq2[search.Hit, error] {
	return func(yield func(search.Hit, error) bool) {
		scope, err := w.source(ctx)
		if err != nil {
			yield(search.Hit{}, err)
			return
		}
		if q.World.IsDefaultWorld() {
			q.World = scope
		}
		for h, err := range w.inner.Search(ctx, q) {
			if !yield(h, err) || err != nil {
				return
			}
		}
	}
}
