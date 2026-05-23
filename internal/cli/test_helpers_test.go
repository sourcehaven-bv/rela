package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/testutil"
)

// testCtx is the cobra-context the test fixture stamps services into.
// Subcommand RunE handlers read services via cli{Read,Write,Analyze}FromContext;
// applySeeder attaches the seeder's bundle here so tests calling RunE(cmd, args)
// with a `cmd` that wraps testCtx pick up the right services.
//
// Sequential by design: there is no `t.Parallel()` in this package yet —
// the cobra-context migration removes the package-global blocker, but
// actually opting tests into parallel is a separate cleanup.
var testCtx context.Context

// fixtureEntities returns every entity of the given type the active
// test fixture knows about. Reads through the test context's read bundle.
func fixtureEntities(entityType string) []*entity.Entity {
	svc := cliReadFromContext(testCtx)
	out := make([]*entity.Entity, 0)
	for e, err := range svc.Store().ListEntities(
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

// fixtureAllEntities returns every entity in the active fixture.
func fixtureAllEntities() []*entity.Entity {
	svc := cliReadFromContext(testCtx)
	out := make([]*entity.Entity, 0)
	for e, err := range svc.Store().ListEntities(context.Background(), store.EntityQuery{}) {
		if err != nil {
			continue
		}
		out = append(out, e)
	}
	return out
}

// fixtureAllRelations returns every relation in the active fixture.
func fixtureAllRelations() []*entity.Relation {
	svc := cliReadFromContext(testCtx)
	out := make([]*entity.Relation, 0)
	for r, err := range svc.Store().ListRelations(context.Background(), store.RelationQuery{}) {
		if err != nil {
			continue
		}
		out = append(out, r)
	}
	return out
}

// storeSeeder is the test-side equivalent of a workspace fixture:
// add entities/relations via seeder, then applySeeder to snapshot
// them into a cliServices bundle and attach it to the test context.
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

// build wraps the seeded store in an *appbuild.Services and returns a
// *cliServices that satisfies cliRead / cliWrite / cliAnalyze. Tests
// share the same appbuild-backed implementation production uses.
func (ss *storeSeeder) build() *cliServices {
	svc, err := newCLIServicesFromAppbuild(
		appbuildtest.New(ss.meta, appbuildtest.WithStore(ss.s)),
	)
	if err != nil {
		panic("storeSeeder.build: " + err.Error())
	}
	return svc
}

// applySeeder snapshots the seeder's store into the package-level
// testCtx so subcommand RunE handlers retrieve services via the
// FromContext accessors. Walks all rooted commands and stamps the
// context on each so tests that call subCmd.RunE(subCmd, args)
// directly pick up the services.
func applySeeder(s *storeSeeder) {
	//nolint:fatcontext // applySeeder seeds a fresh test context; not reading from a passed-in one
	testCtx = attachServices(context.Background(), s.build())
	rootCmd.SetContext(testCtx)
	setContextRecursive(testCtx, rootCmd)
}

func setContextRecursive(ctx context.Context, cmd *cobra.Command) {
	cmd.SetContext(ctx)
	for _, child := range cmd.Commands() {
		setContextRecursive(ctx, child)
	}
}

// testCmd returns a *cobra.Command with the test context attached.
// Pass this into subcommand RunE calls when the test needs to drive
// the handler through the cobra path; the handler will read its
// bundle via cliXFromContext(cmd.Context()). Panics if applySeeder
// hasn't been called yet — that's almost certainly a test setup
// bug (subcommand handler will nil-deref on its bundle accessor).
func testCmd() *cobra.Command {
	if testCtx == nil {
		panic("testCmd: applySeeder must be called before testCmd")
	}
	c := &cobra.Command{}
	c.SetContext(testCtx)
	return c
}
