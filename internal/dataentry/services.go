package dataentry

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
)

// Services bundles the backend services the data-entry handlers read
// from. Each consuming package keeps its own Services type — the coupling
// reduction is worth the field duplication with lua.WriteDeps and friends.
//
// The bundle carries only what HTTP handlers actually need: read-side
// access to the store and tracer, free-text search, and the metamodel.
// Writes continue to flow through workspace methods (which go through
// entitymanager.EntityManager so automations and validations fire).
type Services struct {
	// Store provides entity/relation CRUD. Handlers use it for read
	// operations; writes go through the workspace's EntityManager.
	Store store.Store

	// Tracer walks relations for trace/path/orphan queries.
	Tracer tracer.Tracer

	// Searcher runs free-text queries against the search index.
	Searcher search.Searcher

	// Meta is the current metamodel snapshot.
	Meta *metamodel.Metamodel
}

// Services returns the services bundle.
func (a *App) Services() Services {
	return Services{
		Store:    a.store,
		Tracer:   a.tracer,
		Searcher: a.searcher,
		Meta:     a.State().Meta,
	}
}

// The ungated entity/relation read helpers (getEntity, entityType,
// outgoing/incomingRelations) moved to entityReader (entityreader.go,
// TKT-N26KLB). App reaches them via a.reader.X.

func listRelationsCtx(ctx context.Context, s store.Store, q store.RelationQuery) ([]*entity.Relation, error) {
	out := make([]*entity.Relation, 0)
	for r, err := range s.ListRelations(ctx, q) {
		if err != nil {
			return out, err
		}
		out = append(out, r)
	}
	return out, nil
}
