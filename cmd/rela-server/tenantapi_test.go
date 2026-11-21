package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/jwtauth"
	"github.com/Sourcehaven-BV/rela/internal/tenant"
	"github.com/Sourcehaven-BV/rela/internal/tenanthost"
)

// apiIssuer is a throwaway ES256 issuer with a JWKS server on loopback. It
// returns the JWKS URL and a signer for any claims.
func apiIssuer(t *testing.T) (jwksURL string, sign func(jwt.MapClaims) string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	pub, err := key.PublicKey.ECDH()
	require.NoError(t, err)
	pt := pub.Bytes() // 0x04 || X(32) || Y(32)
	const kid = "tenant-api"
	doc := map[string]any{"keys": []map[string]any{{
		"kty": "EC", "crv": "P-256", "use": "sig", "alg": "ES256", "kid": kid,
		"x": base64.RawURLEncoding.EncodeToString(pt[1:33]),
		"y": base64.RawURLEncoding.EncodeToString(pt[33:65]),
	}}}
	jwks := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(doc)
	}))
	t.Cleanup(jwks.Close)
	return jwks.URL, func(claims jwt.MapClaims) string {
		tok := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
		tok.Header["kid"] = kid
		s, err := tok.SignedString(key)
		require.NoError(t, err)
		return s
	}
}

// TestTenantAPIOperatorAssertion pins the tenant API's authentication
// end to end with a real ES256 issuer and the production assertion
// adapter: the edge's assertion for an operator principal with the role is
// accepted, and an end user's assertion, an operator without the role, or a
// token for another audience is not (TKT-59HWQ76).
func TestTenantAPIOperatorAssertion(t *testing.T) {
	jwksURL, sign := apiIssuer(t)
	ctx := context.Background()
	idv, err := jwtauth.New(ctx, jwtauth.Config{Issuer: "https://edge.test", Audience: "app://rela-pool", JWKSURL: jwksURL})
	require.NoError(t, err)

	ctl := &fakeTenantAPI{rec: tenant.Record{OrgID: "org-a", Status: tenant.StatusActive}}
	opts := &tenantAPIOptions{role: "tenant-manager", reader: ctl, writer: ctl}
	h, err := tenanthost.New(tenanthost.Config{
		Verifier: assertionVerifierAdapter{idv}, HeaderName: "Authorization",
		Acquire: func(context.Context, string) (http.Handler, func(), error) {
			return nil, nil, errors.New("no tenant is opened in this test")
		},
		Static:    http.NotFoundHandler(),
		Provision: tenanthost.ProvisionConfig{Mode: tenanthost.ProvisionLazy, Provisioner: noProvisioner{}},
		TenantAPI: opts.config(func(context.Context, string) {}),
	})
	require.NoError(t, err)
	t.Cleanup(h.Close)

	exp := time.Now().Add(time.Minute).Unix()
	token := func(claims jwt.MapClaims) string {
		c := jwt.MapClaims{
			"iss": "https://edge.test", "aud": "app://rela-pool", "exp": exp, "sub": "harbourmaster",
			"principal_type": "operator", "roles": []string{"tenant-manager"},
		}
		for k, val := range claims {
			if val == nil {
				delete(c, k)
				continue
			}
			c[k] = val
		}
		return sign(c)
	}
	get := func(tok string) int {
		r := httptest.NewRequest(http.MethodGet, tenanthost.TenantsPath+"org-a", http.NoBody)
		r.Header.Set("Authorization", "Bearer "+tok)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}

	require.Equal(t, http.StatusOK, get(token(nil)))
	for name, tc := range map[string]struct {
		claims jwt.MapClaims
		code   int
	}{
		"end user":         {jwt.MapClaims{"principal_type": nil, "org_id": "org-a"}, http.StatusForbidden},
		"user type":        {jwt.MapClaims{"principal_type": "user"}, http.StatusForbidden},
		"operator in org":  {jwt.MapClaims{"org_id": "org-a"}, http.StatusForbidden},
		"other role":       {jwt.MapClaims{"roles": []string{"viewer"}}, http.StatusForbidden},
		"no roles":         {jwt.MapClaims{"roles": nil}, http.StatusForbidden},
		"other audience":   {jwt.MapClaims{"aud": "app://other"}, http.StatusUnauthorized},
		"other issuer":     {jwt.MapClaims{"iss": "https://other.test"}, http.StatusUnauthorized},
		"expired":          {jwt.MapClaims{"exp": time.Now().Add(-time.Hour).Unix()}, http.StatusUnauthorized},
		"empty subject":    {jwt.MapClaims{"sub": ""}, http.StatusUnauthorized},
		"roles as a claim": {jwt.MapClaims{"roles": "tenant-manager"}, http.StatusForbidden},
	} {
		if got := get(token(tc.claims)); got != tc.code {
			t.Errorf("%s: %d, want %d", name, got, tc.code)
		}
	}
}

// noProvisioner is a provisioner no test reaches.
type noProvisioner struct{}

func (noProvisioner) Provision(context.Context, string) error { return errors.New("not reached") }

// fakeTenantAPI is the control table behind the tenant API in these tests.
type fakeTenantAPI struct {
	rec      tenant.Record
	changed  bool
	userType string
}

func (f *fakeTenantAPI) Lookup(context.Context, string) (tenant.Record, error) { return f.rec, nil }

func (f *fakeTenantAPI) Usage(_ context.Context, _, userType string, now time.Time) (tenant.Usage, error) {
	f.userType = userType
	return tenant.Usage{Users: 1, MeasuredAt: now}, nil
}

func (f *fakeTenantAPI) Apply(_ context.Context, _ string, c tenant.Change) (tenant.Record, error) {
	if c.State != nil && *c.State == tenant.StateSuspended {
		f.rec.Status = tenant.StatusSuspended
	}
	return f.rec, nil
}

func (f *fakeTenantAPI) Archive(context.Context, string) (tenant.Record, bool, error) {
	return f.rec, f.changed, nil
}

// TestTenantControl pins what the wiring adds to a tenant API change: a
// lifecycle record under the tenant API's actor, and a revalidation after
// a change of state or an archive so the tenant closes at once.
func TestTenantControl(t *testing.T) {
	dir := t.TempDir()
	ctl := &fakeTenantAPI{rec: tenant.Record{OrgID: "org-a", Schema: "org_x", Status: tenant.StatusActive}, changed: true}
	var revalidated []string
	opts := &tenantAPIOptions{
		role: "tenant-manager", reader: ctl, writer: ctl, userType: "person", log: lifecycleRecorder{cacheDir: dir},
	}
	c := opts.config(func(_ context.Context, org string) { revalidated = append(revalidated, org) }).Control
	ctx := context.Background()

	plan := "team"
	_, err := c.Apply(ctx, "org-a", "harbourmaster", tenant.Change{Plan: &plan})
	require.NoError(t, err)
	require.Empty(t, revalidated, "a plan change does not revalidate")

	st := tenant.StateSuspended
	_, err = c.Apply(ctx, "org-a", "harbourmaster", tenant.Change{State: &st})
	require.NoError(t, err)
	_, err = c.Archive(ctx, "org-a", "harbourmaster")
	require.NoError(t, err)
	require.Equal(t, []string{"org-a", "org-a"}, revalidated)

	u, err := c.Usage(ctx, "org-a")
	require.NoError(t, err)
	require.Equal(t, 1, u.Users)
	require.Equal(t, "person", ctl.userType)

	raw, err := os.ReadFile(filepath.Join(dir, "audit", lifecycleLog))
	require.NoError(t, err)
	var actions []string
	for line := range strings.SplitSeq(strings.TrimSpace(string(raw)), "\n") {
		var rec map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &rec))
		require.Equal(t, actorTenantAPI, rec["operator"])
		require.Equal(t, "harbourmaster", rec["principal"])
		actions = append(actions, rec["action"].(string))
	}
	require.Equal(t, []string{"plan", "state suspended", "archive"}, actions)

	require.Nil(t, (*tenantAPIOptions)(nil).config(nil), "no options is the API off")
}
