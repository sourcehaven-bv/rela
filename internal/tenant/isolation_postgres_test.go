//go:build postgres

// Package tenant_test's isolation suite proves the property the whole design
// rests on, against a real PostgreSQL rather than in prose: a principal
// resolved to one tenant cannot read another tenant's rows.
//
// It is the centrepiece of TKT-TNT9RS. Every other test in this package checks
// bookkeeping — that a lookup fails closed, that eviction closes the right
// store. Those matter, but they would all pass on a design that leaked, because
// the leak would be in the SQL and not in the Go. Only this test looks at the
// database.
package tenant_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/backendtest"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/tenant"
)

const isolationMetamodel = `version: "1.0"
entities:
  ticket:
    label: Ticket
    plural: tickets
    id_prefix: "TKT-"
    id_type: sequential
    properties:
      title:
        type: string
        required: true
relations: {}
`

// TestTenantIsolation_OneTenantCannotReadAnother is acceptance criterion 7 of
// TKT-TNT9RS.
//
// Two tenants, two schemas, one database, one process, one shared
// configuration. An entity written through tenant A's store must be invisible
// through tenant B's, and vice versa — and the reason it is invisible is worth
// stating: rela's SQL is unqualified (`FROM entities`), so it resolves through
// the connection's `search_path`. Tenant B's connection names only tenant B's
// schema, so tenant A's rows are not filtered out, they are unaddressable.
//
// A leak here would not be a missing WHERE clause. It would mean the DSN
// derivation failed to pin `search_path` and both tenants landed on the same
// schema — which is exactly the failure mode that looks like a working
// deployment, and exactly why this assertion is executed rather than asserted.
func TestTenantIsolation_OneTenantCannotReadAnother(t *testing.T) {
	root := writeIsolationProject(t)

	// Two private, migrated schemas on one database. backendtest hands back a
	// DSN per call with search_path pinned, which is the same construction the
	// tenant config derives — so this exercises the real mechanism rather than
	// a test-only shortcut.
	dsnA, dsnB := backendtest.DSN(t), backendtest.DSN(t)
	if dsnA == "" || dsnB == "" {
		t.Fatal("backendtest returned no DSN")
	}
	if dsnA == dsnB {
		t.Fatal("both tenants received the same DSN; the test cannot prove isolation")
	}

	resolver, err := tenant.NewMapResolver([]tenant.Tenant{
		{OrgID: "org-a", Schema: "tenant_a", DSN: dsnA},
		{OrgID: "org-b", Schema: "tenant_b", DSN: dsnB},
	})
	if err != nil {
		t.Fatalf("NewMapResolver: %v", err)
	}

	reg, err := tenant.NewRegistry(resolver, tenant.AppBuildOpener(isolationConfig(t, root)), 4)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	t.Cleanup(func() { _ = reg.Close() })

	ctx := t.Context()
	leaseA, err := reg.Acquire(ctx, "org-a")
	if err != nil {
		t.Fatalf("Acquire(org-a): %v", err)
	}
	defer leaseA.Release()
	leaseB, err := reg.Acquire(ctx, "org-b")
	if err != nil {
		t.Fatalf("Acquire(org-b): %v", err)
	}
	defer leaseB.Release()

	if leaseA.Services() == leaseB.Services() {
		t.Fatal("both tenants were handed the same Services")
	}

	// Write one entity into each tenant, through that tenant's store only.
	secretOfA := &entity.Entity{
		ID:         "TKT-A1",
		Type:       "ticket",
		Properties: map[string]any{"title": "tenant A confidential"},
	}
	secretOfB := &entity.Entity{
		ID:         "TKT-B1",
		Type:       "ticket",
		Properties: map[string]any{"title": "tenant B confidential"},
	}
	if err := leaseA.Services().Store().CreateEntity(ctx, secretOfA); err != nil {
		t.Fatalf("create in tenant A: %v", err)
	}
	if err := leaseB.Services().Store().CreateEntity(ctx, secretOfB); err != nil {
		t.Fatalf("create in tenant B: %v", err)
	}

	// The property: each tenant sees its own row and only its own row.
	assertVisible(t, ctx, leaseA.Services(), "TKT-A1", "tenant A confidential")
	assertVisible(t, ctx, leaseB.Services(), "TKT-B1", "tenant B confidential")
	assertInvisible(t, ctx, leaseA.Services(), "TKT-B1", "A")
	assertInvisible(t, ctx, leaseB.Services(), "TKT-A1", "B")

	// A listing must not leak either. A direct GetEntity could conceivably be
	// gated while a list query still returned foreign rows, so check the bulk
	// read path too — it is the one that would carry a whole tenant's data.
	assertOnlyOwnEntities(t, ctx, leaseA.Services(), "TKT-A1")
	assertOnlyOwnEntities(t, ctx, leaseB.Services(), "TKT-B1")
}

// TestTenantIsolation_UnknownOrgReachesNoDatabase pins the fail-closed rule on
// the path that matters: with a live database configured and reachable, an
// unknown org must still get nothing.
//
// The database-backed version of the same assertion the unit tests make. It is
// worth having separately because the unit tests use a fake opener, so they
// cannot distinguish "denied" from "opened nothing because there was nothing to
// open".
func TestTenantIsolation_UnknownOrgReachesNoDatabase(t *testing.T) {
	root := writeIsolationProject(t)
	dsn := backendtest.DSN(t)

	resolver, err := tenant.NewMapResolver([]tenant.Tenant{
		{OrgID: "org-a", Schema: "tenant_a", DSN: dsn},
	})
	if err != nil {
		t.Fatalf("NewMapResolver: %v", err)
	}
	reg, err := tenant.NewRegistry(resolver, tenant.AppBuildOpener(isolationConfig(t, root)), 4)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	t.Cleanup(func() { _ = reg.Close() })

	for _, orgID := range []string{"", "org-b", "org-a-but-not-quite"} {
		lease, err := reg.Acquire(t.Context(), orgID)
		if !errors.Is(err, tenant.ErrUnknownTenant) {
			if lease != nil {
				lease.Release()
			}
			t.Fatalf("Acquire(%q) error = %v, want ErrUnknownTenant", orgID, err)
		}
		if lease != nil {
			t.Fatalf("Acquire(%q) returned a lease for an unknown org", orgID)
		}
	}
	if reg.Resident() != 0 {
		t.Errorf("Resident() = %d; an unknown org must not open a store", reg.Resident())
	}
}

// TestTenantIsolation_EvictionDoesNotDisturbSiblings runs eviction against real
// stores.
//
// The unit test proves the bookkeeping; this proves the consequence. Closing
// one tenant's Services must tear down only that tenant's pool and search
// closer — if it ever reached something shared, the surviving tenant's reads
// would start failing, which is what this checks.
func TestTenantIsolation_EvictionDoesNotDisturbSiblings(t *testing.T) {
	root := writeIsolationProject(t)

	resolver, err := tenant.NewMapResolver([]tenant.Tenant{
		{OrgID: "org-a", Schema: "tenant_a", DSN: backendtest.DSN(t)},
		{OrgID: "org-b", Schema: "tenant_b", DSN: backendtest.DSN(t)},
	})
	if err != nil {
		t.Fatalf("NewMapResolver: %v", err)
	}
	// A bound of one, so acquiring the second tenant evicts and closes the first.
	reg, err := tenant.NewRegistry(resolver, tenant.AppBuildOpener(isolationConfig(t, root)), 1)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	t.Cleanup(func() { _ = reg.Close() })

	ctx := t.Context()
	leaseA, err := reg.Acquire(ctx, "org-a")
	if err != nil {
		t.Fatalf("Acquire(org-a): %v", err)
	}
	if err := leaseA.Services().Store().CreateEntity(ctx, &entity.Entity{
		ID: "TKT-A1", Type: "ticket", Properties: map[string]any{"title": "a"},
	}); err != nil {
		t.Fatalf("create in tenant A: %v", err)
	}
	leaseA.Release() // now evictable

	leaseB, err := reg.Acquire(ctx, "org-b")
	if err != nil {
		t.Fatalf("Acquire(org-b) after eviction: %v", err)
	}
	defer leaseB.Release()

	if err := leaseB.Services().Store().CreateEntity(ctx, &entity.Entity{
		ID: "TKT-B1", Type: "ticket", Properties: map[string]any{"title": "b"},
	}); err != nil {
		t.Fatalf("tenant B unusable after tenant A was evicted and closed: %v", err)
	}
	assertInvisible(t, ctx, leaseB.Services(), "TKT-A1", "B")

	// Re-acquiring the evicted tenant must reopen it and find its data intact —
	// eviction closes a connection, it does not destroy anything.
	leaseA2, err := reg.Acquire(ctx, "org-a")
	if err != nil {
		t.Fatalf("re-Acquire(org-a): %v", err)
	}
	defer leaseA2.Release()
	assertVisible(t, ctx, leaseA2.Services(), "TKT-A1", "a")
	assertInvisible(t, ctx, leaseA2.Services(), "TKT-B1", "A")
}

// --- helpers ---

func assertVisible(t *testing.T, ctx context.Context, svc *appbuild.Services, id, wantTitle string) {
	t.Helper()
	got, err := svc.Store().GetEntity(ctx, id)
	if err != nil {
		t.Fatalf("tenant cannot read its own entity %s: %v", id, err)
	}
	if title, _ := got.Properties["title"].(string); title != wantTitle {
		t.Errorf("entity %s title = %q, want %q", id, title, wantTitle)
	}
}

// assertInvisible is the leak assertion. A foreign id must not resolve — and
// the failure message says what a failure means, because a green run here is
// the only thing standing between this design and a cross-tenant disclosure.
func assertInvisible(t *testing.T, ctx context.Context, svc *appbuild.Services, id, asTenant string) {
	t.Helper()
	got, err := svc.Store().GetEntity(ctx, id)
	if err == nil {
		t.Fatalf("CROSS-TENANT LEAK: tenant %s read foreign entity %s (%+v); "+
			"search_path is not isolating these tenants", asTenant, id, got.Properties)
	}
}

func assertOnlyOwnEntities(t *testing.T, ctx context.Context, svc *appbuild.Services, wantID string) {
	t.Helper()
	var ids []string
	for e, err := range svc.Store().ListEntities(ctx, store.EntityQuery{Type: "ticket"}) {
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		ids = append(ids, e.ID)
	}
	if len(ids) != 1 || ids[0] != wantID {
		t.Fatalf("CROSS-TENANT LEAK: list returned %v, want only [%s]", ids, wantID)
	}
}

// isolationConfig is the shared, tenant-independent half of construction: one
// project root, one metamodel, one script engine, for every tenant. Only
// DatabaseURL differs per tenant, and AppBuildOpener sets it.
func isolationConfig(t *testing.T, root string) appbuild.Config {
	t.Helper()
	fs := storage.NewSafeFS(storage.NewOsFS())
	paths, err := project.Discover(root, fs)
	if err != nil {
		t.Fatalf("project.Discover: %v", err)
	}
	return appbuild.Config{
		FS:           fs,
		Paths:        paths,
		ScriptEngine: script.NewEngine(),
		Audit:        audit.Nop{},
	}
}

func writeIsolationProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, dir := range []string{"entities", "relations", filepath.Join(".rela", "audit")} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(
		filepath.Join(root, "metamodel.yaml"), []byte(isolationMetamodel), 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}
