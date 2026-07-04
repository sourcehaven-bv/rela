package jwtauth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// the fixed issuer/audience the test JWKS server + verifier agree on.
const (
	testIss = "https://idp.test"
	testAud = "rela"
)

// testIssuer sets up a throwaway ES256 key, a JWKS HTTP server publishing it, and
// a Verifier pointed at that server. It returns the verifier, a mint helper, and
// the server (caller closes it).
func testIssuer(t *testing.T) (*Verifier, func(claims jwt.MapClaims) string, *httptest.Server) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	const kid = "test-key-1"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwksDoc(&key.PublicKey, kid))
	}))

	v, err := New(context.Background(), Config{Issuer: testIss, Audience: testAud, JWKSURL: srv.URL})
	if err != nil {
		srv.Close()
		t.Fatalf("New: %v", err)
	}
	mint := func(claims jwt.MapClaims) string {
		tok := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
		tok.Header["kid"] = kid
		s, err := tok.SignedString(key)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	return v, mint, srv
}

func TestVerifySubject_Valid(t *testing.T) {
	v, mint, srv := testIssuer(t)
	defer srv.Close()

	tok := mint(jwt.MapClaims{
		"iss": "https://idp.test", "aud": "rela", "sub": "usr_abc",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	sub, err := v.VerifySubject(context.Background(), tok)
	if err != nil || sub != "usr_abc" {
		t.Fatalf("VerifySubject = %q, %v; want usr_abc, nil", sub, err)
	}
}

func TestVerifySubject_Rejects(t *testing.T) {
	v, mint, srv := testIssuer(t)
	defer srv.Close()

	cases := []struct {
		name   string
		claims jwt.MapClaims
	}{
		{"wrong issuer", jwt.MapClaims{"iss": "https://evil.test", "aud": "rela", "sub": "u", "exp": future()}},
		{"wrong audience", jwt.MapClaims{"iss": "https://idp.test", "aud": "other", "sub": "u", "exp": future()}},
		{"expired", jwt.MapClaims{"iss": "https://idp.test", "aud": "rela", "sub": "u", "exp": time.Now().Add(-time.Hour).Unix()}},
		{"no expiry", jwt.MapClaims{"iss": "https://idp.test", "aud": "rela", "sub": "u"}},
		{"empty subject", jwt.MapClaims{"iss": "https://idp.test", "aud": "rela", "sub": "", "exp": future()}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := v.VerifySubject(context.Background(), mint(c.claims)); !errors.Is(err, ErrInvalid) {
				t.Errorf("expected ErrInvalid, got %v", err)
			}
		})
	}
}

func TestVerifySubject_RejectsGarbageAndEmpty(t *testing.T) {
	v, _, srv := testIssuer(t)
	defer srv.Close()
	for _, raw := range []string{"", "not.a.jwt", "a.b.c"} {
		if _, err := v.VerifySubject(context.Background(), raw); !errors.Is(err, ErrInvalid) {
			t.Errorf("raw %q: expected ErrInvalid, got %v", raw, err)
		}
	}
}

// TestVerifySubject_RejectsAlgNone guards against the classic alg:none bypass —
// an unsigned token with a valid claim set must be refused.
func TestVerifySubject_RejectsAlgNone(t *testing.T) {
	v, _, srv := testIssuer(t)
	defer srv.Close()
	tok := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{
		"iss": "https://idp.test", "aud": "rela", "sub": "attacker", "exp": future(),
	})
	raw, _ := tok.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if _, err := v.VerifySubject(context.Background(), raw); !errors.Is(err, ErrInvalid) {
		t.Errorf("alg:none must be rejected, got %v", err)
	}
}

// TestVerifySubject_RejectsForgedKey is the key false-ACCEPT guard: a token with
// a VALID ES256 signature — but signed by a key that is NOT in the JWKS — must be
// rejected. This is the case a naive verifier gets wrong; garbage-string tests
// don't exercise it.
func TestVerifySubject_RejectsForgedKey(t *testing.T) {
	v, _, srv := testIssuer(t)
	defer srv.Close()

	// A second, independent key the JWKS never published.
	attacker, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"iss": testIss, "aud": testAud, "sub": "victim", "exp": future(),
	})
	tok.Header["kid"] = "test-key-1"       // claim the real kid...
	raw, err := tok.SignedString(attacker) // ...but sign with the wrong key
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.VerifySubject(context.Background(), raw); !errors.Is(err, ErrInvalid) {
		t.Errorf("a well-signed token from a non-JWKS key must be rejected, got %v", err)
	}
}

// TestVerifySubject_RejectsRS256 pins alg-confusion beyond alg:none: an RS256
// token must be refused (only ES256 is accepted).
func TestVerifySubject_RejectsRS256(t *testing.T) {
	v, _, srv := testIssuer(t)
	defer srv.Close()
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss": testIss, "aud": testAud, "sub": "u", "exp": future(),
	})
	raw, _ := tok.SignedString(rsaKey)
	if _, err := v.VerifySubject(context.Background(), raw); !errors.Is(err, ErrInvalid) {
		t.Errorf("RS256 must be rejected (ES256 only), got %v", err)
	}
}

func future() int64 { return time.Now().Add(time.Hour).Unix() }

// TestNew_FatalOnUnreachableJWKS pins the C1 contract: New MUST error when the
// JWKS can't be fetched at startup, so the server fails loud instead of booting a
// verifier that silently rejects every token. (keyfunc's default would swallow
// this; we override NoErrorReturnFirstHTTPReq=false.)
func TestNew_FatalOnUnreachableJWKS(t *testing.T) {
	// A server that's immediately closed → the URL is unreachable.
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	url := srv.URL
	srv.Close()

	if _, err := New(context.Background(), Config{Issuer: testIss, Audience: testAud, JWKSURL: url}); err == nil {
		t.Fatal("New must error when the JWKS is unreachable at startup")
	}
}

// TestNew_RejectsNonHTTPS pins the C2 contract: a non-https JWKS URL is refused
// (the JWKS is the root of trust; cleartext would be MITM-able). Loopback http is
// exempt so local/test servers work.
func TestNew_RejectsNonHTTPS(t *testing.T) {
	_, err := New(context.Background(), Config{Issuer: testIss, Audience: testAud, JWKSURL: "http://idp.example.com/jwks"})
	if err == nil {
		t.Fatal("New must reject a non-https, non-loopback JWKS URL")
	}
}

// TestNew_RequiresAllPins: issuer, audience, and jwks url are all mandatory.
func TestNew_RequiresAllPins(t *testing.T) {
	cases := []Config{
		{Audience: "a", JWKSURL: "https://x/jwks"}, // no issuer
		{Issuer: "i", JWKSURL: "https://x/jwks"},   // no audience
		{Issuer: "i", Audience: "a"},               // no jwks
	}
	for _, c := range cases {
		if _, err := New(context.Background(), c); err == nil {
			t.Errorf("New(%+v) should require all pins", c)
		}
	}
}

// jwksDoc builds a minimal JWKS document publishing one EC P-256 public key. It
// derives the x/y coordinates from the key's uncompressed point (0x04||X||Y),
// avoiding the deprecated big.Int coordinate accessors.
func jwksDoc(pub *ecdsa.PublicKey, kid string) map[string]any {
	ep, err := pub.ECDH()
	if err != nil {
		panic(err)
	}
	pt := ep.Bytes() // 0x04 || X(32) || Y(32) for P-256
	x, y := pt[1:33], pt[33:65]
	return map[string]any{
		"keys": []map[string]any{{
			"kty": "EC", "crv": "P-256", "use": "sig", "alg": "ES256", "kid": kid,
			"x": base64.RawURLEncoding.EncodeToString(x),
			"y": base64.RawURLEncoding.EncodeToString(y),
		}},
	}
}
