package mcp

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/attachment"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// newTestDeps assembles the [Deps] the server consumes from an
// appbuildtest services bundle, so the test wiring is single-sourced
// with the rest of the repo instead of hand-mirroring the production
// cli.mcpServices graph (TKT-R2KBG6 — the previous hand-rolled wiring
// is where the nil-templater booby trap of TKT-TLQ94B lived).
//
// The bundle wires an in-memory bleve backend and backfills it from
// the caller-supplied store, so entities seeded before construction
// are searchable; writes during the test do not reach the index
// (same semantics as the previous fixture).
func newTestDeps(t *testing.T, meta *metamodel.Metamodel, st store.Store) Deps {
	t.Helper()

	svc := appbuildtest.New(meta, appbuildtest.WithStore(st))
	t.Cleanup(func() { _ = svc.Close() })

	return Deps{
		Store:         svc.GatedReads().Reader,
		Traversals:    svc.GatedReads().Traversals,
		Meta:          meta,
		Tracer:        svc.Tracer(),
		Searcher:      svc.Searcher(),
		Validator:     svc.Validator(),
		EntityManager: svc.EntityManager(),
		Config:        svc.Config(),
		LuaWriteDeps:  svc.LuaWriteDeps(),
		Watcher:       nopWatcher{},
		// Note: this real empty dir (what lua_run walks) is
		// intentionally distinct from LuaWriteDeps' ProjectRoot, which
		// points at the fixture's in-memory /project. No current test
		// resolves a Lua write relative to that root; align the two if
		// one ever does.
		ProjectRoot: t.TempDir(),
		Attachments: testAttachmentDeps(t, svc, meta, audit.Nop{}),
		World:       store.TrivialScope(),
		Families:    store.TrivialScope(),
	}
}

// testAttachmentDeps wires the attachment tools over svc the way `rela mcp`
// does (no command runner, store backstop limit), with sink as the audit log.
func testAttachmentDeps(
	t *testing.T, svc *appbuild.Services, meta *metamodel.Metamodel, sink audit.Audit,
) AttachmentDeps {
	t.Helper()
	owner, err := entitymanager.AttachmentsOf(svc.EntityManager())
	if err != nil {
		t.Fatalf("AttachmentsOf: %v", err)
	}
	snap, err := NewAttachmentSnapshot(svc.Store(), owner, svc.ACL(), meta, nil, store.MaxAttachmentBytes)
	if err != nil {
		t.Fatalf("NewAttachmentSnapshot: %v", err)
	}
	return AttachmentDeps{
		Snapshot:   func() (AttachmentSnapshot, error) { return snap, nil },
		Uploads:    attachment.NewLimiter(attachment.DefaultMaxUploads),
		Authorizer: svc.ACL(),
		Audit:      sink,
	}
}

// nopWatcher satisfies the narrow watcher interface MCP consumes;
// tests never exercise file watching.
type nopWatcher struct{}

func (nopWatcher) Start(func()) error { return nil }
func (nopWatcher) Stop()              {}
func (nopWatcher) Pause()             {}
func (nopWatcher) Resume()            {}

// graphOf is the ungated [GraphReader] over st, as `rela mcp` wires it with
// no acl.yaml: rows through [visibility.Unrestricted], counts and single
// relations from the store.
func graphOf(st store.Store) GraphReader {
	return rawGraph{UnrestrictedReader: visibility.Unrestricted(st), st: st}
}

type rawGraph struct {
	*visibility.UnrestrictedReader
	st store.Store
}

func (g rawGraph) Resolve(ctx context.Context, addr string) (*entity.Entity, error) {
	return g.GetAddress(ctx, addr)
}

func (g rawGraph) GetRelation(ctx context.Context, k entity.RelationKey) (*entity.Relation, error) {
	return g.st.GetRelation(ctx, k)
}

func (g rawGraph) CountEntities(ctx context.Context, q store.EntityQuery) (int, error) {
	return g.st.CountEntities(ctx, q)
}

func (g rawGraph) CountRelations(ctx context.Context, q store.RelationQuery) (int, error) {
	return g.st.CountRelations(ctx, q)
}
