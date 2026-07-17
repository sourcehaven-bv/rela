package cli

import (
	"bytes"
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/testutil"
)

// storeSeeder builds the CLI service bundles around an in-memory store
// the test populates via addEntity/addRelation. Same shape the
// production code uses — newCLIBundles wraps an appbuild.Services that
// itself wraps the seeded memstore.
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

func (ss *storeSeeder) build(t *testing.T) *cliBundles {
	t.Helper()
	b, err := newCLIBundles(
		appbuildtest.New(ss.meta, appbuildtest.WithStore(ss.s)),
	)
	if err != nil {
		t.Fatalf("storeSeeder.build: %v", err)
	}
	return b
}

// fixtureEntities returns every entity of the given type from st.
func fixtureEntities(t *testing.T, st store.Store, entityType string) []*entity.Entity {
	t.Helper()
	out := make([]*entity.Entity, 0)
	for e, err := range st.ListEntities(
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

// fixtureAllEntities returns every entity in st.
func fixtureAllEntities(t *testing.T, st store.Store) []*entity.Entity {
	t.Helper()
	out := make([]*entity.Entity, 0)
	for e, err := range st.ListEntities(context.Background(), store.EntityQuery{}) {
		if err != nil {
			continue
		}
		out = append(out, e)
	}
	return out
}

// fixtureAllRelations returns every relation in st.
func fixtureAllRelations(t *testing.T, st store.Store) []*entity.Relation {
	t.Helper()
	out := make([]*entity.Relation, 0)
	for r, err := range st.ListRelations(context.Background(), store.RelationQuery{}) {
		if err != nil {
			continue
		}
		out = append(out, r)
	}
	return out
}

// withOutput captures the test's stdout into a buffer. Restores the
// previous out writer on test cleanup. Use the returned buffer to
// assert on rendered output.
func withOutput(t *testing.T, format output.Format) *bytes.Buffer {
	t.Helper()
	buf := new(bytes.Buffer)
	prev := out
	out = output.NewWithWriter(buf, format)
	t.Cleanup(func() { out = prev })
	return buf
}
