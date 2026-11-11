//go:build postgres

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/backendtest"
	"github.com/Sourcehaven-BV/rela/internal/dataentry"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/tenant"
	"github.com/Sourcehaven-BV/rela/internal/tenanthost"
)

const tenantSchema = `version: "1.0"
entities:
  task:
    label: Task
    plural: tasks
    id_prefix: TSK
    id_type: sequential
    properties:
      title:
        type: string
        required: true
`

const tenantACL = `roles:
  editor:
    read: [task]
    create: [task]
    update: [task]
assignments:
  ada: editor
`

// orgVerifier stands in for jwtauth: "tok-<org>" verifies as ada in that
// org. Everything else is rejected.
type orgVerifier struct{}

func (orgVerifier) VerifyAssertion(_ context.Context, raw string) (dataentry.AssertedIdentity, error) {
	org, ok := strings.CutPrefix(raw, "tok-")
	if !ok {
		return dataentry.AssertedIdentity{}, errors.New("bad signature")
	}
	return dataentry.AssertedIdentity{Subject: "ada", OrgID: org}, nil
}

// tenantDeployment is a tenant-mode deployment over a private database:
// two orgs created with `rela-server tenant create`, as in production.
type tenantDeployment struct {
	dir, base string
	env       func(string) string
}

// newTenantDeployment writes the project and creates org-a and org-b
// through the tenant command.
func newTenantDeployment(t *testing.T) tenantDeployment {
	t.Helper()
	dir := t.TempDir()
	for name, body := range map[string]string{
		"schema.yaml":     tenantSchema,
		"data-entry.yaml": "app:\n  name: Tasks\n",
		"acl.yaml":        tenantACL,
	} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600))
	}
	base := backendtest.Database(t)
	d := tenantDeployment{dir: dir, base: base, env: func(k string) string {
		if k == "RELA_DATABASE_URL" {
			return base
		}
		return ""
	}}
	for _, org := range []string{"org-a", "org-b"} {
		code, _, stderr := d.run(t, "", "create", "-project", dir, org)
		require.Equal(t, 0, code, stderr)
	}
	return d
}

// run runs `rela-server tenant ARGS` against the deployment.
func (d tenantDeployment) run(t *testing.T, stdin string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	tio := productionTenantIO()
	tio.in, tio.out, tio.err, tio.getenv = strings.NewReader(stdin), &out, &errOut, d.env
	tio.user = func() string { return "tester" }
	code = runTenant(t.Context(), args, tio)
	return code, out.String(), errOut.String()
}

// host builds tenant mode over the deployment, with the production wiring
// except for the JWKS-backed verifier.
func (d tenantDeployment) host(t *testing.T) *tenantHost {
	t.Helper()
	f := &serverFlags{projectDir: d.dir, bind: "127.0.0.1", port: "8080", jwtHeader: "X-Auth-Assertion"}
	shared, err := appbuild.DiscoverSharedBase(d.dir, script.NewEngine(), discoverOptions(f)...)
	require.NoError(t, err)
	resolver, err := tenant.NewClusterResolver(t.Context(), d.base)
	require.NoError(t, err)
	require.NoError(t, resolver.Check(t.Context()))

	s := &server{f: f, addr: "127.0.0.1:8080", mode: identityJWT, assertion: tenanthost.ReuseAssertion(orgVerifier{})}
	th, err := s.tenantHost(shared, resolver, tenantLimits{resident: 4}, orgVerifier{})
	require.NoError(t, err)
	th.resolver = resolver
	t.Cleanup(th.close)
	return th
}

// newTestTenantHost builds tenant mode over two freshly created tenants.
func newTestTenantHost(t *testing.T) *tenantHost {
	t.Helper()
	return newTenantDeployment(t).host(t)
}

func tenantCall(t *testing.T, th *tenantHost, token, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Host = th.addr
	req.Header.Set("Origin", "http://"+th.addr)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("X-Auth-Assertion", token)
	}
	rec := httptest.NewRecorder()
	th.handler.ServeHTTP(rec, req)
	return rec
}

// TestTenantMode_TenantsCannotSeeEachOther is the acceptance test of
// TKT-D6MF3SW: through the HTTP API, with the production wiring, a task one
// tenant writes is invisible to the other, in a read by id, a list and a
// search.
func TestTenantMode_TenantsCannotSeeEachOther(t *testing.T) {
	th := newTestTenantHost(t)

	create := func(token, title string) string {
		rec := tenantCall(t, th, token, http.MethodPost, "/api/v1/tasks",
			map[string]any{"properties": map[string]any{"title": title}})
		require.Contains(t, []int{http.StatusOK, http.StatusCreated}, rec.Code, rec.Body.String())
		var out struct {
			ID string `json:"id"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
		require.NotEmpty(t, out.ID, rec.Body.String())
		return out.ID
	}
	idA := create("tok-org-a", "alpha secret")
	create("tok-org-b", "beta secret")

	// Read by id: A's task exists for A and not for B.
	require.Equal(t, http.StatusOK,
		tenantCall(t, th, "tok-org-a", http.MethodGet, "/api/v1/tasks/"+idA, nil).Code)

	for _, tc := range []struct{ token, own, foreign string }{
		{"tok-org-a", "alpha secret", "beta secret"},
		{"tok-org-b", "beta secret", "alpha secret"},
	} {
		list := tenantCall(t, th, tc.token, http.MethodGet, "/api/v1/tasks", nil)
		require.Equal(t, http.StatusOK, list.Code, list.Body.String())
		require.Contains(t, list.Body.String(), tc.own)
		require.NotContains(t, list.Body.String(), tc.foreign, "CROSS-TENANT LEAK in the list")

		search := tenantCall(t, th, tc.token, http.MethodGet, "/api/v1/_search?q=secret", nil)
		require.Equal(t, http.StatusOK, search.Code, search.Body.String())
		require.NotContains(t, search.Body.String(), tc.foreign, "CROSS-TENANT LEAK in search")
	}

	// The two tasks got ids from separate sequences, so the same id can name
	// a different task in B; what must hold is that B never sees A's title.
	other := tenantCall(t, th, "tok-org-b", http.MethodGet, "/api/v1/tasks/"+idA, nil)
	require.NotContains(t, other.Body.String(), "alpha secret", "CROSS-TENANT LEAK by id")
}

// TestTenantMode_FailsClosedOverRealTenants checks the denials with real
// tenants behind the host: none opens a store.
func TestTenantMode_FailsClosedOverRealTenants(t *testing.T) {
	th := newTestTenantHost(t)
	require.Equal(t, http.StatusUnauthorized,
		tenantCall(t, th, "", http.MethodGet, "/api/v1/tasks", nil).Code)
	require.Equal(t, http.StatusUnauthorized,
		tenantCall(t, th, "forged", http.MethodGet, "/api/v1/tasks", nil).Code)
	require.Equal(t, http.StatusForbidden,
		tenantCall(t, th, "tok-org-z", http.MethodGet, "/api/v1/tasks", nil).Code)
	require.Equal(t, http.StatusForbidden,
		tenantCall(t, th, "tok-", http.MethodGet, "/api/v1/tasks", nil).Code)
	require.Equal(t, 0, th.registry.Resident(), "a denial opened a tenant")

	// The shell loads without a tenant.
	require.Equal(t, http.StatusOK, tenantCall(t, th, "", http.MethodGet, "/", nil).Code)
	require.Equal(t, 0, th.registry.Resident())
}

// TestTenantMode_CreateServeSuspendErase is the lifecycle acceptance test of
// TKT-HF5KE12: tenants made with `tenant create` are served, a suspended
// one is refused and closed, and an erased one is gone with its data, while
// the other tenant is untouched.
func TestTenantMode_CreateServeSuspendErase(t *testing.T) {
	d := newTenantDeployment(t)
	th := d.host(t)
	post := func(token string) int {
		return tenantCall(t, th, token, http.MethodPost, "/api/v1/tasks",
			map[string]any{"properties": map[string]any{"title": "x"}}).Code
	}
	require.Contains(t, []int{http.StatusOK, http.StatusCreated}, post("tok-org-a"))
	require.Contains(t, []int{http.StatusOK, http.StatusCreated}, post("tok-org-b"))

	code, out, stderr := d.run(t, "", "show", "-project", d.dir, "org-a")
	require.Equal(t, 0, code, stderr)
	require.Contains(t, out, "active")

	// Suspend: the next request is refused, and revalidation closes it.
	code, _, stderr = d.run(t, "", "suspend", "-project", d.dir, "org-a")
	require.Equal(t, 0, code, stderr)
	require.Equal(t, http.StatusForbidden, tenantCall(t, th, "tok-org-a", http.MethodGet, "/api/v1/tasks", nil).Code)
	th.registry.Revalidate(t.Context())
	require.Equal(t, 1, th.registry.Resident(), "the suspended tenant stayed open")

	// Erasing is refused while a server holds the tenant; org-b is open, so
	// erasing it first fails and records the attempt.
	code, _, stderr = d.run(t, "", "erase", "-project", d.dir, "-force", "org-b")
	require.Equal(t, 1, code)
	require.Contains(t, stderr, "locked")
	code, _, _ = d.run(t, "", "resume", "-project", d.dir, "org-b")
	require.Equal(t, 1, code, "an erasing tenant was resumed")

	// Erase org-a with the typed confirmation.
	schemaA, err := tenant.SchemaFor("org-a")
	require.NoError(t, err)
	auditA := filepath.Join(tenant.AuditRoot(filepath.Join(d.dir, ".rela")), schemaA)
	require.NoError(t, os.MkdirAll(auditA, 0o750))
	code, out, stderr = d.run(t, "org-a\n", "erase", "-project", d.dir, "org-a")
	require.Equal(t, 0, code, stderr)
	require.Contains(t, out, "dropped")
	require.NotContains(t, out+stderr, d.base, "the output carries the DSN")
	_, err = os.Stat(auditA)
	require.ErrorIs(t, err, os.ErrNotExist)
	code, out, _ = d.run(t, "", "show", "-project", d.dir, "org-a")
	require.Equal(t, 0, code)
	require.Contains(t, out, "no tenant")
	require.Equal(t, http.StatusForbidden, tenantCall(t, th, "tok-org-a", http.MethodGet, "/api/v1/tasks", nil).Code)

	rec, err := os.ReadFile(filepath.Join(d.dir, ".rela", "audit", erasureLog))
	require.NoError(t, err)
	require.Contains(t, string(rec), `"ok":true`)
	require.Contains(t, string(rec), `"ok":false`)

	// org-b was never erased: its row is erasing, so it is no longer served.
	// The host closes it on revalidation, and the erase can then finish.
	th.registry.Revalidate(t.Context())
	require.Equal(t, 0, th.registry.Resident())
	code, _, stderr = d.run(t, "", "erase", "-project", d.dir, "-force", "org-b")
	require.Equal(t, 0, code, stderr)
}

// TestTenantCommand_CreateRefusals pins the create refusals over a real
// database: a second create, and a schema that exists without a row.
func TestTenantCommand_CreateRefusals(t *testing.T) {
	d := newTenantDeployment(t)
	code, _, stderr := d.run(t, "", "create", "-project", d.dir, "org-a")
	require.Equal(t, 1, code)
	require.Contains(t, stderr, "already exists")

	schema, err := tenant.SchemaFor("org-c")
	require.NoError(t, err)
	conn, err := pgx.Connect(t.Context(), d.base)
	require.NoError(t, err)
	defer func() { _ = conn.Close(context.Background()) }()
	_, err = conn.Exec(t.Context(), "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize())
	require.NoError(t, err)
	code, _, stderr = d.run(t, "", "create", "-project", d.dir, "org-c")
	require.Equal(t, 1, code)
	require.Contains(t, stderr, "-adopt")
	code, _, stderr = d.run(t, "", "create", "-project", d.dir, "-adopt", "org-c")
	require.Equal(t, 0, code, stderr)
}

// TestRefuseTenantDeployment_TenantDatabase pins that a server without
// tenant mode refuses a database that serves tenants.
func TestRefuseTenantDeployment_TenantDatabase(t *testing.T) {
	d := newTenantDeployment(t)
	err := refuseTenantDeployment(t.Context(), d.dir, d.env)
	require.ErrorIs(t, err, tenant.ErrTenantDatabase)
}

// TestTenantMode_RevokeRacesLastRelease runs a revalidation against the
// last release of a suspended tenant on a real resident: the tenant closes,
// and after a resume it is served by a fresh resident. Revoke and Close may
// both end the app's streams; tenantResident.retire makes that happen once.
func TestTenantMode_RevokeRacesLastRelease(t *testing.T) {
	d := newTenantDeployment(t)
	th := d.host(t)
	for range 3 {
		lease, err := th.registry.Acquire(t.Context(), "org-a")
		require.NoError(t, err)
		code, _, stderr := d.run(t, "", "suspend", "-project", d.dir, "org-a")
		require.Equal(t, 0, code, stderr)

		var wg sync.WaitGroup
		wg.Go(func() { th.registry.Revalidate(t.Context()) })
		wg.Go(lease.Release)
		wg.Wait()
		require.Eventually(t, func() bool { return th.registry.Resident() == 0 }, 5*time.Second, 10*time.Millisecond)

		code, _, stderr = d.run(t, "", "resume", "-project", d.dir, "org-a")
		require.Equal(t, 0, code, stderr)
		// The revoked resident may still be closing; the next acquire
		// opens a fresh one once it has.
		require.Eventually(t, func() bool {
			l, err := th.registry.Acquire(t.Context(), "org-a")
			if err != nil {
				return false
			}
			l.Release()
			return true
		}, 5*time.Second, 20*time.Millisecond)
	}
}
