package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/jwtauth"
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

// TestTenantAPIVerifier pins the tenant API's token check against a real
// ES256 issuer: only the tenant API audience is accepted, and an end
// user's assertion or a webhook token is not (TKT-84CD0PY).
func TestTenantAPIVerifier(t *testing.T) {
	jwksURL, sign := apiIssuer(t)
	ctx := context.Background()
	idv, err := jwtauth.New(ctx, jwtauth.Config{Issuer: "https://idp.test", Audience: "rela", JWKSURL: jwksURL})
	require.NoError(t, err)

	f := jwtFlags()
	off, err := buildTenantAPIVerifier(f, idv)
	require.NoError(t, err)
	require.Nil(t, off, "no audience leaves the API off")

	f.tenant.apiAudience = "rela-tenant-api"
	v, err := buildTenantAPIVerifier(f, idv)
	require.NoError(t, err)

	exp := time.Now().Add(time.Minute).Unix()
	token := func(claims jwt.MapClaims) string {
		c := jwt.MapClaims{"iss": "https://idp.test", "aud": "rela-tenant-api", "exp": exp, "org_id": "org-a"}
		for k, val := range claims {
			if val == nil {
				delete(c, k)
				continue
			}
			c[k] = val
		}
		return sign(c)
	}

	got, err := v.VerifyTenantAPI(ctx, token(jwt.MapClaims{"jti": "j1"}))
	require.NoError(t, err)
	require.Equal(t, "org-a", got.OrgID)
	require.Equal(t, "j1", got.ID)

	for name, claims := range map[string]jwt.MapClaims{
		"user assertion": {"aud": "rela"},
		"webhook token":  {"aud": "rela-webhook"},
		"other issuer":   {"iss": "https://other.test"},
		"no expiry":      {"exp": nil},
		"expired":        {"exp": time.Now().Add(-time.Hour).Unix()},
	} {
		if _, err := v.VerifyTenantAPI(ctx, token(claims)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
