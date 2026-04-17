package dataentry

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
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

	// Search runs a free-text query against the Bleve index. The
	// words/phrases split mirrors the existing workspace.Search signature
	// — unifying it with search.Searcher is a separate piece of work.
	Search SearchFunc

	// Meta is the current metamodel snapshot.
	Meta *metamodel.Metamodel
}

// SearchFunc executes a free-text search against the workspace's search
// index. words match loosely; phrases match as contiguous token sequences.
// limit <= 0 means no limit. Returns entities in relevance order, paired
// with their relevance scores.
type SearchFunc func(words, phrases []string, limit int) ([]*model.Entity, []float64, error)

// Services returns the services bundle the current App state is wired to.
// Services are read from the workspace at call time, so reloads that
// swap the backing store surface here immediately.
func (a *App) Services() Services {
	return Services{
		Store:  a.ws.Store(),
		Tracer: a.ws.Tracer(),
		Search: func(words, phrases []string, limit int) ([]*model.Entity, []float64, error) {
			entities, scores, err := a.ws.Search(words, phrases, limit)
			if err != nil {
				return nil, nil, err
			}
			out := make([]*model.Entity, len(entities))
			for i, e := range entities {
				out[i] = model.EntityFromDomain(e)
			}
			return out, scores, nil
		},
		Meta: a.State().Meta,
	}
}

// getEntityAsModel looks up an entity by ID via the store and converts it
// to *model.Entity. Drop-in replacement for the old s.Graph.GetNode shape,
// so the migration can proceed without simultaneously flipping every
// handler to *entity.Entity. Remove once the flip is complete.
func (a *App) getEntityAsModel(id string) (*model.Entity, bool) {
	e, err := a.ws.Store().GetEntity(context.Background(), id)
	if err != nil {
		return nil, false
	}
	return model.EntityFromDomain(e), true
}

// outgoingRelationsAsModel returns all outgoing relations for id as
// []*model.Relation — the pre-migration shape. See getEntityAsModel.
func (a *App) outgoingRelationsAsModel(id string) []*model.Relation {
	return listRelationsAsModel(a.ws.Store(), store.RelationQuery{
		EntityID:  id,
		Direction: store.DirectionOutgoing,
	})
}

// incomingRelationsAsModel returns all incoming relations for id as
// []*model.Relation.
func (a *App) incomingRelationsAsModel(id string) []*model.Relation {
	return listRelationsAsModel(a.ws.Store(), store.RelationQuery{
		EntityID:  id,
		Direction: store.DirectionIncoming,
	})
}

func listRelationsAsModel(s store.Store, q store.RelationQuery) []*model.Relation {
	var out []*model.Relation
	for r, err := range s.ListRelations(context.Background(), q) {
		if err != nil {
			return out
		}
		out = append(out, model.RelationFromDomain(r))
	}
	return out
}
