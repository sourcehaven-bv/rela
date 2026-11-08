//go:build postgres

package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/backendtest"
	"github.com/Sourcehaven-BV/rela/internal/dataentry"
	"github.com/Sourcehaven-BV/rela/internal/peerkey"
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

// newTestTenantHost builds tenant mode over two real schemas, with the
// production wiring except for the JWKS-backed verifier.
func newTestTenantHost(t *testing.T) *tenantHost {
	t.Helper()
	dir := t.TempDir()
	for name, body := range map[string]string{
		"schema.yaml":     tenantSchema,
		"data-entry.yaml": "app:\n  name: Tasks\n",
		"acl.yaml":        tenantACL,
	} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600))
	}
	f := &serverFlags{projectDir: dir, bind: "127.0.0.1", port: "8080", jwtHeader: "X-Auth-Assertion"}
	base, err := appbuild.DiscoverSharedBase(dir, script.NewEngine(), discoverOptions(f)...)
	require.NoError(t, err)
	dsnA, schemaA := backendtest.SchemaDSN(t)
	dsnB, schemaB := backendtest.SchemaDSN(t)
	resolver, err := tenant.NewMapResolver([]tenant.Tenant{
		{OrgID: "org-a", Schema: schemaA, DSN: dsnA},
		{OrgID: "org-b", Schema: schemaB, DSN: dsnB},
	})
	require.NoError(t, err)

	s := &server{f: f, addr: "127.0.0.1:8080", mode: identityJWT, assertion: tenanthost.ReuseAssertion(orgVerifier{})}
	th, err := s.tenantHost(base, resolver, tenantLimits{resident: 4}, orgVerifier{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = th.registry.Close() })
	return th
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

// TestTenantMode_IgnoresPeerKeys pins that tenant apps have no peer keys.
// Every tenant shares the project's secrets, so a wired peer key would be
// one credential valid in every tenant. A peer-shaped Authorization value
// next to a valid assertion is ignored, where a wired edge would refuse an
// unknown key with 401.
func TestTenantMode_IgnoresPeerKeys(t *testing.T) {
	th := newTestTenantHost(t)
	key, _, err := peerkey.Generate(rand.Reader)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil)
	req.Host = th.addr
	req.Header.Set("Origin", "http://"+th.addr)
	req.Header.Set("X-Auth-Assertion", "tok-org-a")
	req.Header.Set("Authorization", "Bearer "+key)
	rec := httptest.NewRecorder()
	th.handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}
