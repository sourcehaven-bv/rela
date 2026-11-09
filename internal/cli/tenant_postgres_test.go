//go:build postgres

package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/backendtest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/tenant"
)

// The tests in this file are the acceptance tests of TKT-M3SHYE0: two
// tenants on one database, one shared config, and each operator entry point
// acting in exactly the tenant it names.

const tenantEntrySchema = `entities:
  note:
    label: Note
    id_type: short
    id_prefix: "NOTE-"
    properties:
      title: {type: string, required: true}
relations: {}
`

// writeTwoTenantProject writes a project with a scheduled task that creates
// one note, and a tenant map with two tenants on private schemas of the test
// database. The map names schemas only; the base DSN comes from
// $RELA_DATABASE_URL as in production, so no credential is in the file.
func writeTwoTenantProject(t *testing.T) (root, schemaA, schemaB string) {
	t.Helper()
	_, schemaA = backendtest.SchemaDSN(t)
	_, schemaB = backendtest.SchemaDSN(t)
	base := os.Getenv("RELA_TEST_DATABASE_URL")
	require.NotEmpty(t, base)
	t.Setenv("RELA_DATABASE_URL", base)

	root = t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "scripts"), 0o750))
	for name, body := range map[string]string{
		"schema.yaml":      tenantEntrySchema,
		"schedules.yaml":   "tasks:\n  - name: tick\n    script: tick.lua\n    every: 1h\n",
		"scripts/tick.lua": `rela.create_entity("note", { title = "scheduled" })`,
		tenant.ConfigFileName: fmt.Sprintf("tenants:\n  - {org_id: org-a, schema: %s}\n  - {org_id: org-b, schema: %s}\n",
			schemaA, schemaB),
	} {
		require.NoError(t, os.WriteFile(filepath.Join(root, name), []byte(body), 0o600))
	}
	return root, schemaA, schemaB
}

func openTenantForTest(t *testing.T, root, org string) *appbuild.Services {
	t.Helper()
	svc, closer, err := openProject(root, org)
	require.NoError(t, err)
	t.Cleanup(func() { _ = closer.Close() })
	return svc
}

func noteTitles(t *testing.T, svc *appbuild.Services) []string {
	t.Helper()
	var out []string
	for e, err := range svc.Store().ListEntities(context.Background(),
		store.EntityQuery{Type: "note", Faces: store.InWorld(store.TrivialScope())}) {
		require.NoError(t, err)
		title, _ := e.Properties["title"].(string)
		out = append(out, title)
	}
	return out
}

// TestTenantScheduler_WritesOnlyIntoItsTenant is acceptance criterion 1:
// `rela --tenant org-a scheduler` runs org-a's schedule, and the note it
// writes, the run state it records and the audit records it leaves are all
// in org-a. org-b sees none of them.
func TestTenantScheduler_WritesOnlyIntoItsTenant(t *testing.T) {
	root, schemaA, schemaB := writeTwoTenantProject(t)
	svcA := openTenantForTest(t, root, "org-a")
	svcB := openTenantForTest(t, root, "org-b")
	require.Equal(t, schemaA, searchPathSchema(t, svcA), "org-a's services land in org-a's schema")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	bind, err := appbuild.IdentityBinder(svcA)
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() { done <- (&SchedulerCmd{}).Run(ctx, svcA, bind) }()

	settled := func() bool {
		got, loadErr := svcA.SchedulerState().Load(context.Background(), []string{"tick"})
		if loadErr != nil {
			return false
		}
		ts, ok := got["tick"]
		return ok && ts.Active == nil && !ts.LastRun.IsZero()
	}
	require.Eventually(t, settled, 10*time.Second, 20*time.Millisecond, "org-a's task did not run")
	cancel()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("scheduler did not stop")
	}

	assert.Equal(t, []string{"scheduled"}, noteTitles(t, svcA), "the task writes into the tenant it ran for")
	assert.Empty(t, noteTitles(t, svcB), "the task must not write into another tenant")

	stateB, err := svcB.SchedulerState().Load(context.Background(), []string{"tick"})
	require.NoError(t, err)
	assert.Empty(t, stateB, "run state must stay in the tenant's own schema")

	// The run's audit records go to org-a's sink only.
	cacheDir := svcA.Paths().CacheDir
	entriesA, _ := os.ReadDir(filepath.Join(tenant.AuditRoot(cacheDir), schemaA))
	assert.NotEmpty(t, entriesA, "the write is audited in org-a's sink")
	entriesB, _ := os.ReadDir(filepath.Join(tenant.AuditRoot(cacheDir), schemaB))
	assert.Empty(t, entriesB, "org-b's sink records nothing for org-a's run")
	_, err = os.Stat(filepath.Join(cacheDir, "audit", time.Now().Format("2006-01-02")+".jsonl"))
	assert.ErrorIs(t, err, os.ErrNotExist, "nothing is audited in the single-store sink")
}

// searchPathSchema returns the first schema of the services' search_path.
func searchPathSchema(t *testing.T, svc *appbuild.Services) string {
	t.Helper()
	cfg, err := pgx.ParseConfig(svc.Base().Config().DatabaseURL)
	require.NoError(t, err)
	schema := cfg.RuntimeParams["search_path"]
	for i, r := range schema {
		if r == ',' {
			return schema[:i]
		}
	}
	return schema
}

// TestTenantMCP_SeesOnlyItsTenant pins acceptance criterion 2 for stdio MCP:
// a session opened for org-b reads org-b's store, not org-a's.
func TestTenantMCP_SeesOnlyItsTenant(t *testing.T) {
	root, _, _ := writeTwoTenantProject(t)
	svcA := openTenantForTest(t, root, "org-a")

	bind, err := appbuild.IdentityBinder(svcA)
	require.NoError(t, err)
	ctx, err := bind(principal.With(context.Background(),
		principal.Principal{User: "operator", Tool: principal.ToolCLI}))
	require.NoError(t, err)
	note := entity.New("", "note")
	note.SetString("title", "alpha")
	_, err = svcA.EntityManager().CreateEntity(ctx, note, entity.CreateOptions{})
	require.NoError(t, err)

	mcpB, err := newMCPServices(root, "org-b", "")
	require.NoError(t, err)
	t.Cleanup(func() { _ = mcpB.Close() })
	assert.Empty(t, noteTitles(t, mcpB.current()), "an MCP session for org-b must not see org-a's notes")

	mcpA, err := newMCPServices(root, "org-a", "")
	require.NoError(t, err)
	t.Cleanup(func() { _ = mcpA.Close() })
	assert.Equal(t, []string{"alpha"}, noteTitles(t, mcpA.current()))
}

// TestTenantMCP_CloseReleasesTheSchemaLock pins that closing an MCP session
// releases its tenant's schema lock: erasure takes the lock exclusively and
// is refused while any session holds it shared.
func TestTenantMCP_CloseReleasesTheSchemaLock(t *testing.T) {
	root, schemaA, _ := writeTwoTenantProject(t)
	mcpA, err := newMCPServices(root, "org-a", "")
	require.NoError(t, err)

	held := func() bool {
		conn, err := pgx.Connect(context.Background(), os.Getenv("RELA_TEST_DATABASE_URL"))
		require.NoError(t, err)
		defer func() { _ = conn.Close(context.Background()) }()
		var n int
		require.NoError(t, conn.QueryRow(context.Background(),
			`SELECT count(*) FROM pg_locks WHERE locktype = 'advisory' AND mode = 'ShareLock'
			   AND classid = $1::oid AND objid = hashtext($2)::oid`,
			int64(0x72656c61), schemaA).Scan(&n))
		return n > 0
	}
	require.True(t, held(), "an open MCP session holds its tenant's schema lock")
	// The pin is a socket, which Go closes on garbage collection, so a leaked
	// pin would release the lock eventually anyway. Check the closer is the
	// tenant's, and allow the backend only a moment to exit.
	_, isTenant := mcpA.closeOrigin.(*tenant.Opened)
	require.True(t, isTenant, "a tenant session must close through the tenant.Opened")
	require.NoError(t, mcpA.Close())
	require.Eventually(t, func() bool { return !held() }, 500*time.Millisecond, 10*time.Millisecond,
		"closing the session must release the schema lock")
}

// TestTenantDB_RefusesAMissingSchema pins that `rela db migrate` in tenant
// mode checks the tenant's schema before it migrates. A schema that does not
// exist is skipped by search_path, so an unchecked migration would create
// rela's tables in public.
func TestTenantDB_RefusesAMissingSchema(t *testing.T) {
	db := backendtest.Database(t)
	t.Setenv("RELA_DATABASE_URL", db)
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "schema.yaml"), []byte(tenantEntrySchema), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, tenant.ConfigFileName),
		[]byte("tenants:\n  - {org_id: org-a, schema: never_created}\n"), 0o600))

	oldPath, oldOrg := projectPath, tenantOrg
	projectPath, tenantOrg = root, "org-a"
	t.Cleanup(func() { projectPath, tenantOrg = oldPath, oldOrg })

	err := runDBMigrate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "never_created")

	conn, err := pgx.Connect(context.Background(), db)
	require.NoError(t, err)
	defer func() { _ = conn.Close(context.Background()) }()
	var n int
	require.NoError(t, conn.QueryRow(context.Background(),
		"SELECT count(*) FROM pg_tables WHERE schemaname = 'public'").Scan(&n))
	assert.Zero(t, n, "a refused migration must leave public without tables")

	tenantOrg = ""
	require.ErrorIs(t, runDBMigrate(), tenant.ErrTenantRequired)
}

// TestTenantDB_TargetsTheNamedProject pins that a db command takes the
// project, and so the schema its indexes are derived from, from --project
// rather than from the working directory.
func TestTenantDB_TargetsTheNamedProject(t *testing.T) {
	root, _, _ := writeTwoTenantProject(t)
	t.Chdir(t.TempDir())

	db, err := resolveDSN(root, "org-a")
	require.NoError(t, err)
	defer db.release()
	require.NotEmpty(t, db.root)
	want, err := filepath.EvalSymlinks(root)
	require.NoError(t, err)
	got, err := filepath.EvalSymlinks(db.root)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

// TestDB_RefusesATenantDatabaseOutsideAProject pins that `rela db migrate`
// run outside the project directory, where it cannot see tenants.yaml,
// refuses a database that already serves tenants. Its $RELA_DATABASE_URL is
// then the base DSN, and migrating it would put rela's tables in public.
func TestDB_RefusesATenantDatabaseOutsideAProject(t *testing.T) {
	db := backendtest.Database(t)
	t.Setenv("RELA_DATABASE_URL", db)
	t.Chdir(t.TempDir())
	oldPath, oldOrg := projectPath, tenantOrg
	projectPath, tenantOrg = "", ""
	t.Cleanup(func() { projectPath, tenantOrg = oldPath, oldOrg })

	conn, err := pgx.Connect(context.Background(), db)
	require.NoError(t, err)
	defer func() { _ = conn.Close(context.Background()) }()
	publicTables := func() int {
		var n int
		require.NoError(t, conn.QueryRow(context.Background(),
			"SELECT count(*) FROM pg_tables WHERE schemaname = 'public'").Scan(&n))
		return n
	}

	_, err = conn.Exec(context.Background(),
		"CREATE SCHEMA tenant_x; CREATE TABLE tenant_x.schema_version (version int)")
	require.NoError(t, err)
	require.ErrorIs(t, runDBMigrate(), tenant.ErrLooksLikeTenantDatabase)
	assert.Zero(t, publicTables(), "a refused migration must leave public without tables")

	// A single-store database is migrated outside a project as before.
	_, err = conn.Exec(context.Background(), "DROP SCHEMA tenant_x CASCADE")
	require.NoError(t, err)
	require.NoError(t, runDBMigrate())
	assert.NotZero(t, publicTables(), "a single-store database is migrated in its own schema")
}

// TestTenantOpen_ErrorNamesNoPassword pins that a failed tenant open does
// not echo the base DSN's password: the connect error is wrapped with the
// org id only, and pgx redacts the password from its own message.
func TestTenantOpen_ErrorNamesNoPassword(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "schema.yaml"), []byte(tenantEntrySchema), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, tenant.ConfigFileName),
		[]byte("tenants:\n  - {org_id: org-a, schema: tenant_a}\n"), 0o600))
	const secret = "s3cret-pw"
	t.Setenv("RELA_DATABASE_URL", "postgres://u:"+secret+"@tenant-db.invalid/rela?connect_timeout=1")

	_, _, err := openProject(root, "org-a")
	require.Error(t, err)
	assert.NotContains(t, err.Error(), secret)
	assert.Contains(t, err.Error(), "org-a")
}
