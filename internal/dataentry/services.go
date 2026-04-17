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
// reduction is worth the field duplication with lua.Services and friends.
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

// Services returns the services bundle the current App state is wired to.
// Services are read from the workspace at call time, so reloads that
// swap the backing store surface here immediately.
func (a *App) Services() Services {
	return Services{
		Store:    a.ws.Store(),
		Tracer:   a.ws.Tracer(),
		Searcher: a.ws.Searcher(),
		Meta:     a.State().Meta,
	}
}

// getEntity looks up an entity by ID via the store.
func (a *App) getEntity(id string) (*entity.Entity, bool) {
	e, err := a.ws.Store().GetEntity(context.Background(), id)
	if err != nil {
		return nil, false
	}
	return e, true
}

// outgoingRelations returns all outgoing relations for id.
func (a *App) outgoingRelations(id string) []*entity.Relation {
	return listRelations(a.ws.Store(), store.RelationQuery{
		EntityID:  id,
		Direction: store.DirectionOutgoing,
	})
}

// incomingRelations returns all incoming relations for id.
func (a *App) incomingRelations(id string) []*entity.Relation {
	return listRelations(a.ws.Store(), store.RelationQuery{
		EntityID:  id,
		Direction: store.DirectionIncoming,
	})
}

func listRelations(s store.Store, q store.RelationQuery) []*entity.Relation {
	out := make([]*entity.Relation, 0)
	for r, err := range s.ListRelations(context.Background(), q) {
		if err != nil {
			return out
		}
		out = append(out, r)
	}
	return out
}
