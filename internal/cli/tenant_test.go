package cli

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild/backendtest"
	"github.com/Sourcehaven-BV/rela/internal/tenant"
)

// tenantDSN points at a host that does not exist. A test that reaches a
// tenant lookup with it fails with a connection error, never a selection.
const tenantDSN = "postgres://rela@tenant-db.invalid/rela?connect_timeout=1"

// tenantEnv switches tenant mode on over tenantDSN.
func tenantEnv(k string) string {
	switch k {
	case tenant.EnvTenantMode:
		return "1"
	case "RELA_DATABASE_URL":
		return tenantDSN
	}
	return ""
}

// setTenantEnv puts tenantEnv into the process environment, for a path that
// reads it with os.Getenv.
func setTenantEnv(t *testing.T) {
	t.Helper()
	t.Setenv(tenant.EnvTenantMode, "1")
	t.Setenv("RELA_DATABASE_URL", tenantDSN)
}

// seedLegacyProject is seedProject with the old tenant map file.
func seedLegacyProject(t *testing.T) string {
	t.Helper()
	root := seedProject(t)
	require.NoError(t, os.WriteFile(filepath.Join(root, "tenants.yaml"), []byte("tenants: []\n"), 0o600))
	return root
}

func noEnv(string) string { return "" }

// TestCheckTenantMode pins the CLI's tenant gate: without tenant mode only a
// named tenant is refused; in tenant mode every data command must name a
// tenant, and the tenant-free commands pass either way. A project with the
// old tenant map is refused in both. Which tenants are served is pinned
// against a real control table in tenant_postgres_test.go.
func TestCheckTenantMode(t *testing.T) {
	t.Parallel()

	single := seedProject(t)
	legacy := seedLegacyProject(t)
	nested := filepath.Join(single, "entities")

	cases := []struct {
		name, cmd, dir, org string
		env                 func(string) string
		// want is the error on a build that serves tenant mode; nil means the
		// gate passes.
		want error
		// tenantMode marks a case whose outcome needs a tenant-mode build:
		// on any other build the gate refuses it as unsupported.
		tenantMode bool
	}{
		{name: "single store, no flag", cmd: "list", dir: single, env: noEnv},
		{name: "single store, flag", cmd: "list", dir: single, org: "org-a", env: noEnv, want: tenant.ErrNotTenantMode},
		{name: "single store, tenant-free with flag", cmd: "version", dir: single, org: "org-a", env: noEnv},
		{name: "no project", cmd: "list", dir: t.TempDir(), env: noEnv},
		{name: "legacy map", cmd: "list", dir: legacy, env: noEnv, want: tenant.ErrLegacyTenantMap},
		{name: "legacy map in tenant mode", cmd: "list", dir: legacy, org: "org-a", env: tenantEnv,
			want: tenant.ErrLegacyTenantMap},
		{name: "tenant mode, no flag", cmd: "list", dir: single, env: tenantEnv, want: tenant.ErrTenantRequired,
			tenantMode: true},
		{name: "tenant mode, from a subdirectory", cmd: "mcp", dir: nested, env: tenantEnv,
			want: tenant.ErrTenantRequired, tenantMode: true},
		{name: "tenant mode, db", cmd: "db migrate", dir: single, env: tenantEnv, want: tenant.ErrTenantRequired,
			tenantMode: true},
		{name: "tenant mode, tenant-free", cmd: "init", dir: single, env: tenantEnv},
		{name: "tenant mode, validate", cmd: "validate", dir: single, env: tenantEnv},
		{name: "tenant mode, config migrate", cmd: "migrate", dir: single, env: tenantEnv},
		{name: "tenant mode, data migrate", cmd: "migrate data", dir: single, env: tenantEnv,
			want: tenant.ErrTenantRequired, tenantMode: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := checkTenantMode(tc.cmd, tc.dir, tc.org, tc.env)
			if tc.tenantMode && !tenant.Supported {
				require.ErrorIs(t, err, tenant.ErrUnsupportedBuild, "a build without tenant mode must refuse it")
				return
			}
			if tc.want == nil {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, tc.want)
		})
	}
}

// TestCheckTenantMode_EveryCommandIsGated walks every command the CLI
// declares and checks that, in tenant mode without a tenant, the gate
// refuses all of them except the reviewed tenant-free set. A new command is
// therefore gated by default.
func TestCheckTenantMode_EveryCommandIsGated(t *testing.T) {
	t.Parallel()

	parser, err := kong.New(&CLI{}, kong.Vars{"version": "test"})
	require.NoError(t, err)
	root := seedProject(t)

	free := map[string]bool{}
	var walk func(n *kong.Node)
	walk = func(n *kong.Node) {
		for _, child := range n.Children {
			if child.Type != kong.CommandNode {
				continue
			}
			path := child.Path()
			if len(child.Children) > 0 {
				walk(child)
			}
			if err := checkTenantMode(path, root, "", tenantEnv); err == nil {
				free[firstKongToken(path)] = true
			}
		}
	}
	walk(parser.Model.Node)

	assert.Equal(t, map[string]bool{
		"version": true, "init": true, "completion": true, "apps": true, "migrate": true, "validate": true,
	}, free, "only these commands may run in tenant mode without a tenant; "+
		"a command that reads or writes data must be gated")
	for name := range tenantFreeCommands {
		assert.True(t, free[name], "tenantFreeCommands lists %q, which the CLI does not declare", name)
	}
}

// TestOpenProject_SingleTenantUnchanged pins that a project without tenant
// mode opens exactly as appbuild.Discover opens it, and that naming a tenant
// there is refused rather than ignored.
func TestOpenProject_SingleTenantUnchanged(t *testing.T) {
	root := seedProject(t)
	t.Setenv(tenant.EnvTenantMode, "")
	for k, v := range backendtest.Env(t) {
		t.Setenv(k, v)
	}

	svc, closer, err := openProject(root, "")
	require.NoError(t, err)
	t.Cleanup(func() { _ = closer.Close() })
	assert.Same(t, svc, closer, "a single-store project is closed through its services")
	abs, err := filepath.EvalSymlinks(root)
	require.NoError(t, err)
	got, err := filepath.EvalSymlinks(svc.Paths().Root)
	require.NoError(t, err)
	assert.Equal(t, abs, got)

	_, _, err = openProject(root, "org-a")
	require.ErrorIs(t, err, tenant.ErrNotTenantMode)
}

// TestOpenProject_TenantModeFailsClosed pins that tenant mode opens nothing
// for a missing tenant, or one whose lookup fails. The base DSN points at a
// host that does not exist, so no lookup can succeed.
func TestOpenProject_TenantModeFailsClosed(t *testing.T) {
	root := seedProject(t)
	setTenantEnv(t)

	for _, org := range []string{"", "org-c"} {
		svc, closer, err := openProject(root, org)
		require.Error(t, err, "org %q", org)
		assert.Nil(t, svc)
		assert.Nil(t, closer)
		if !tenant.Supported {
			require.ErrorIs(t, err, tenant.ErrUnsupportedBuild)
			continue
		}
		if org == "" {
			require.ErrorIs(t, err, tenant.ErrTenantRequired)
		} else {
			assert.NotErrorIs(t, err, tenant.ErrUnknownTenant, "a failed lookup is not an unknown tenant")
		}
	}
}

// TestNewMCPServices_TenantModeFailsClosed pins that stdio MCP goes through
// the same selection as every other command.
func TestNewMCPServices_TenantModeFailsClosed(t *testing.T) {
	root := seedProject(t)
	setTenantEnv(t)

	for _, org := range []string{"", "org-c"} {
		_, err := newMCPServices(root, org, "")
		require.Error(t, err, "org %q", org)
	}
}

// TestOpenProject_RefusesLegacyMap pins that a project with the old tenant
// map opens nothing, with or without tenant mode.
func TestOpenProject_RefusesLegacyMap(t *testing.T) {
	root := seedLegacyProject(t)
	t.Setenv(tenant.EnvTenantMode, "")
	_, _, err := openProject(root, "")
	require.ErrorIs(t, err, tenant.ErrLegacyTenantMap)
}

// TestCLIOpensProjectsOnlyThroughOpenProject scans this package for a way
// to reach project data that skips tenant selection: an appbuild constructor
// that opens a project, a pgstore DSN helper, or the database URL itself.
// Each file that may use one is listed with the reason.
func TestCLIOpensProjectsOnlyThroughOpenProject(t *testing.T) {
	t.Parallel()

	banned := map[string]map[string]bool{
		"appbuild": {"Discover": true, "At": true, "DiscoverSharedBase": true, "New": true, "NewSharedBase": true},
		"pgstore":  {"MigrateDSN": true, "StatusDSN": true, "ReconcileDSN": true},
	}
	allowed := map[string]string{
		"tenant.go":      "openProject itself",
		"db_postgres.go": "the db commands, through resolveDSN",
		"mcp_wiring.go":  "a schema reload rebuilds over the session's own (tenant) config",
	}

	files, err := filepath.Glob("*.go")
	require.NoError(t, err)
	fset := token.NewFileSet()
	for _, name := range files {
		if _, ok := allowed[name]; ok || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		require.NoError(t, err)
		ast.Inspect(f, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.SelectorExpr:
				if pkg, ok := n.X.(*ast.Ident); ok && banned[pkg.Name][n.Sel.Name] {
					t.Errorf("%s: %s.%s reaches project data without tenant selection; use openProject",
						fset.Position(n.Pos()), pkg.Name, n.Sel.Name)
				}
			case *ast.BasicLit:
				if n.Kind == token.STRING && strings.Contains(n.Value, "RELA_DATABASE_URL") {
					t.Errorf("%s: reads the database URL without tenant selection; use openProject or resolveDSN",
						fset.Position(n.Pos()))
				}
			}
			return true
		})
	}
}

// TestValidate_TenantModeWithoutTenant pins that `rela validate` checks the
// shared config in tenant mode without a tenant, and that its entity
// checks, which read data, still need one.
func TestValidate_TenantModeWithoutTenant(t *testing.T) {
	root := seedProject(t)
	setTenantEnv(t)
	oldPath, oldOrg := projectPath, tenantOrg
	projectPath, tenantOrg = root, ""
	t.Cleanup(func() { projectPath, tenantOrg = oldPath, oldOrg })

	require.NoError(t, (&ValidateCmd{}).Run(t.Context()), "config validation reads no tenant data")

	err := (&ValidateCmd{Check: []string{"cardinality"}}).Run(t.Context())
	require.Error(t, err, "entity checks must not run without a tenant")
	if tenant.Supported {
		require.ErrorIs(t, err, tenant.ErrTenantRequired)
	} else {
		require.ErrorIs(t, err, tenant.ErrUnsupportedBuild)
	}
}
