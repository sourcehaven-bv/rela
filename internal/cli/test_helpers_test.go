package cli

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/testutil"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

// fixtureEntities returns every entity of the given type the active
// workspace knows about. Use this to gather test inputs for rendering
// helpers (DOT, CSV, etc.) instead of reaching into the legacy graph.
func fixtureEntities(entityType string) []*entity.Entity {
	out := make([]*entity.Entity, 0)
	for e, err := range ws.Store().ListEntities(
		context.Background(),
		store.EntityQuery{Type: entityType},
	) {
		if err != nil {
			continue
		}
		out = append(out, e)
	}
	return out
}

// fixtureAllEntities returns every entity in the active workspace.
func fixtureAllEntities() []*entity.Entity {
	out := make([]*entity.Entity, 0)
	for e, err := range ws.Store().ListEntities(context.Background(), store.EntityQuery{}) {
		if err != nil {
			continue
		}
		out = append(out, e)
	}
	return out
}

// fixtureAllRelations returns every relation in the active workspace.
func fixtureAllRelations() []*entity.Relation {
	out := make([]*entity.Relation, 0)
	for r, err := range ws.Store().ListRelations(context.Background(), store.RelationQuery{}) {
		if err != nil {
			continue
		}
		out = append(out, r)
	}
	return out
}

// storeSeeder is the test-side equivalent of a workspace fixture:
// add entities/relations via seeder, then applySeeder to snapshot
// them into a workspace.Workspace backed by the store. Replaces the
// legacy "build a graph, pass to workspace, mutate the graph further"
// pattern that coupled tests to graph internals.
type storeSeeder struct {
	s    store.Store
	meta *metamodel.Metamodel
}

func newStoreSeeder(meta *metamodel.Metamodel) *storeSeeder {
	return &storeSeeder{s: memstore.New(), meta: meta}
}

func (ss *storeSeeder) addEntity(b *testutil.EntityBuilder) {
	e := b.Build()
	if err := ss.s.CreateEntity(context.Background(), e); err != nil {
		panic(err)
	}
}

func (ss *storeSeeder) addRelation(from, relType, to string) {
	if _, err := ss.s.CreateRelation(context.Background(), from, relType, to, nil); err != nil {
		panic(err)
	}
}

func (ss *storeSeeder) build() *workspace.Workspace {
	return workspace.NewForTest(ss.meta, workspace.WithTestStore(ss.s))
}

// applySeeder snapshots the seeder's store into the package-level ws
// global that the CLI command handlers read.
func applySeeder(s *storeSeeder) {
	ws = s.build()
}
