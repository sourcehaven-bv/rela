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

// getEntity looks up an entity by ID via the store.
func (a *App) getEntity(ctx context.Context, id string) (*entity.Entity, bool) {
	e, err := a.store.GetEntity(ctx, id)
	if err != nil {
		return nil, false
	}
	return e, true
}

// peerType returns the entity type for the peer ID on the other end of
// a relation edge, or empty string if the peer can't be resolved. Used
// by the relation GET handlers to emit a `type` field per edge so SPA
// clients can construct JSON:API §9 resource identifiers without
// guessing.
func (a *App) peerType(ctx context.Context, id string) string {
	if e, ok := a.getEntity(ctx, id); ok {
		return e.Type
	}
	return ""
}

// outgoingRelations returns all outgoing relations for id. Iterator
// errors are swallowed; use outgoingRelationsCtx to surface them.
func (a *App) outgoingRelations(ctx context.Context, id string) []*entity.Relation {
	rels, _ := a.outgoingRelationsCtx(ctx, id)
	return rels
}

// outgoingRelationsCtx returns all outgoing relations for id and surfaces
// iterator errors rather than silently truncating the slice.
func (a *App) outgoingRelationsCtx(ctx context.Context, id string) ([]*entity.Relation, error) {
	return listRelationsCtx(ctx, a.store, store.RelationQuery{
		EntityID:  id,
		Direction: store.DirectionOutgoing,
	})
}

// incomingRelations returns all incoming relations for id.
func (a *App) incomingRelations(ctx context.Context, id string) []*entity.Relation {
	rels, _ := listRelationsCtx(ctx, a.store, store.RelationQuery{
		EntityID:  id,
		Direction: store.DirectionIncoming,
	})
	return rels
}

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
