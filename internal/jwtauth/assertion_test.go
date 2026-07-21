package jwtauth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// VerifyAssertion — the widened identity projection (TKT-RP3X3Q).
//
// The claim shapes here mirror what Pratique actually mints (see
// pratique/internal/signer/signer.go): org_id and org_slug are strings, roles
// is an array of bare strings, and every one of them is always present.

// jsonNull marks an override that should set a claim to an explicit JSON null,
// as distinct from deleting the key. A plain nil in the override map deletes.
var jsonNull = struct{}{}

// assertionClaims builds a valid claim set, with overrides applied on top.
func assertionClaims(over map[string]any) jwt.MapClaims {
	c := jwt.MapClaims{
		"iss":      testIss,
		"aud":      testAud,
		"sub":      "usr_abc123",
		"email":    "alice@example.com",
		"org_id":   "org_acme",
		"org_slug": "acme",
		"roles":    []any{"admin", "billing"},
		"exp":      time.Now().Add(time.Hour).Unix(),
		"iat":      time.Now().Unix(),
	}
	for k, v := range over {
		switch v {
		case nil:
			delete(c, k)
		case any(jsonNull):
			c[k] = nil // serializes as an explicit JSON null
		default:
			c[k] = v
		}
	}
	return c
}

func TestVerifyAssertion_ProjectsAllClaims(t *testing.T) {
	v, mint, srv := testIssuer(t)
	defer srv.Close()

	got, err := v.VerifyAssertion(context.Background(), mint(assertionClaims(nil)))
	if err != nil {
		t.Fatalf("VerifyAssertion: %v", err)
	}

	if got.Subject != "usr_abc123" {
		t.Errorf("Subject = %q, want usr_abc123", got.Subject)
	}
	if got.Email != "alice@example.com" {
		t.Errorf("Email = %q", got.Email)
	}
	if got.OrgID != "org_acme" {
		t.Errorf("OrgID = %q, want org_acme", got.OrgID)
	}
	if got.OrgSlug != "acme" {
		t.Errorf("OrgSlug = %q, want acme", got.OrgSlug)
	}
	if want := []string{"admin", "billing"}; !slices.Equal(got.Roles, want) {
		t.Errorf("Roles = %v, want %v", got.Roles, want)
	}
}

func TestVerifyAssertion_AbsentClaimsAreNotAnError(t *testing.T) {
	v, mint, srv := testIssuer(t)
	defer srv.Close()

	for _, tc := range []struct {
		name      string
		over      map[string]any
		wantRoles []string
		wantOrg   string
	}{
		{"no roles key", map[string]any{"roles": nil}, nil, "org_acme"},
		{"empty roles array", map[string]any{"roles": []any{}}, nil, "org_acme"},
		{"explicit null roles", map[string]any{"roles": jsonNull}, nil, "org_acme"},
		{
			"no org",
			map[string]any{"org_id": nil, "org_slug": nil},
			[]string{"admin", "billing"}, "",
		},
		{
			"subject only",
			map[string]any{"email": nil, "org_id": nil, "org_slug": nil, "roles": nil},
			nil, "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := v.VerifyAssertion(context.Background(), mint(assertionClaims(tc.over)))
			if err != nil {
				t.Fatalf("VerifyAssertion: %v", err)
			}
			if got.Subject != "usr_abc123" {
				t.Errorf("Subject = %q, want the assertion to still verify", got.Subject)
			}
			if !slices.Equal(got.Roles, tc.wantRoles) {
				t.Errorf("Roles = %v, want %v", got.Roles, tc.wantRoles)
			}
			if got.OrgID != tc.wantOrg {
				t.Errorf("OrgID = %q, want %q", got.OrgID, tc.wantOrg)
			}
		})
	}
}

func TestVerifyAssertion_MalformedClaimsAreTreatedAsAbsent(t *testing.T) {
	v, mint, srv := testIssuer(t)
	defer srv.Close()

	for _, tc := range []struct {
		name      string
		over      map[string]any
		wantRoles []string
	}{
		{"roles is a string, not an array", map[string]any{"roles": "admin"}, nil},
		{"roles is an object", map[string]any{"roles": map[string]any{"a": 1}}, nil},
		{
			"non-string elements are skipped",
			map[string]any{"roles": []any{"admin", 42, nil, "billing"}},
			[]string{"admin", "billing"},
		},
		{"org_id is a number", map[string]any{"org_id": 42}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := v.VerifyAssertion(context.Background(), mint(assertionClaims(tc.over)))
			if err != nil {
				t.Fatalf("a malformed claim must not fail an otherwise valid assertion: %v", err)
			}
			if tc.wantRoles != nil && !slices.Equal(got.Roles, tc.wantRoles) {
				t.Errorf("Roles = %v, want %v", got.Roles, tc.wantRoles)
			}
		})
	}
}

func TestVerifyAssertion_BoundsRoles(t *testing.T) {
	v, mint, srv := testIssuer(t)
	defer srv.Close()

	// A verified signature proves the IdP asserted these values, not that they
	// are small. Without a cap, one request becomes N attribution entries and a
	// proportionally huge audit line.
	many := make([]any, 0, maxRoles*3)
	for range maxRoles * 3 {
		many = append(many, "role")
	}
	got, err := v.VerifyAssertion(context.Background(),
		mint(assertionClaims(map[string]any{"roles": many})))
	if err != nil {
		t.Fatalf("VerifyAssertion: %v", err)
	}
	if len(got.Roles) > maxRoles {
		t.Errorf("len(Roles) = %d, want <= %d", len(got.Roles), maxRoles)
	}

	// Per-element length is capped too.
	long := strings.Repeat("x", maxRoleRunes*3)
	got, err = v.VerifyAssertion(context.Background(),
		mint(assertionClaims(map[string]any{"roles": []any{long}})))
	if err != nil {
		t.Fatalf("VerifyAssertion: %v", err)
	}
	if n := len([]rune(got.Roles[0])); n > maxRoleRunes {
		t.Errorf("role length = %d runes, want <= %d", n, maxRoleRunes)
	}
}

func TestVerifyAssertion_MultibyteRoleNotSplit(t *testing.T) {
	v, mint, srv := testIssuer(t)
	defer srv.Close()

	// Truncation must not slice a multi-byte character in half.
	role := strings.Repeat("é", maxRoleRunes*2)
	got, err := v.VerifyAssertion(context.Background(),
		mint(assertionClaims(map[string]any{"roles": []any{role}})))
	if err != nil {
		t.Fatalf("VerifyAssertion: %v", err)
	}
	if !strings.HasPrefix(role, got.Roles[0]) {
		t.Error("truncation split a multi-byte rune")
	}
}

func TestVerifyAssertion_RejectsInvalid(t *testing.T) {
	v, mint, srv := testIssuer(t)
	defer srv.Close()

	// The security property: claims are projected ONLY from a token that passed
	// every verification step. Each case must yield ErrInvalid and zero claims.
	forged, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	mintForged := func(c jwt.MapClaims) string {
		tok := jwt.NewWithClaims(jwt.SigningMethodES256, c)
		tok.Header["kid"] = "test-key-1"
		s, sErr := tok.SignedString(forged)
		if sErr != nil {
			t.Fatal(sErr)
		}
		return s
	}
	mintNone := func(c jwt.MapClaims) string {
		tok := jwt.NewWithClaims(jwt.SigningMethodNone, c)
		s, sErr := tok.SignedString(jwt.UnsafeAllowNoneSignatureType)
		if sErr != nil {
			t.Fatal(sErr)
		}
		return s
	}

	for _, tc := range []struct {
		name  string
		token string
	}{
		{"forged key", mintForged(assertionClaims(nil))},
		{"alg none", mintNone(assertionClaims(nil))},
		{"wrong issuer", mint(assertionClaims(map[string]any{"iss": "https://evil.test"}))},
		{"wrong audience", mint(assertionClaims(map[string]any{"aud": "someone-else"}))},
		{"expired", mint(assertionClaims(map[string]any{
			"exp": time.Now().Add(-time.Hour).Unix(),
		}))},
		{"no expiry", mint(assertionClaims(map[string]any{"exp": nil}))},
		{"empty subject", mint(assertionClaims(map[string]any{"sub": ""}))},
		{"no subject", mint(assertionClaims(map[string]any{"sub": nil}))},
		{"garbage", "not.a.jwt"},
		{"empty", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := v.VerifyAssertion(context.Background(), tc.token)
			if !errors.Is(err, ErrInvalid) {
				t.Errorf("err = %v, want ErrInvalid", err)
			}
			if got.Subject != "" || got.OrgID != "" || len(got.Roles) != 0 {
				t.Errorf("claims leaked from an unverified token: %+v", got)
			}
		})
	}
}

func TestVerifyAssertion_MatchesVerifySubject(t *testing.T) {
	v, mint, srv := testIssuer(t)
	defer srv.Close()

	// VerifyAssertion widens VerifySubject rather than replacing it; the two
	// must agree on identity and on what they reject.
	for _, tc := range []struct {
		name  string
		token string
	}{
		{"valid", mint(assertionClaims(nil))},
		{"expired", mint(assertionClaims(map[string]any{
			"exp": time.Now().Add(-time.Hour).Unix(),
		}))},
		{"garbage", "not.a.jwt"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sub, subErr := v.VerifySubject(context.Background(), tc.token)
			got, asrErr := v.VerifyAssertion(context.Background(), tc.token)

			if (subErr == nil) != (asrErr == nil) {
				t.Errorf("disagreement: VerifySubject err=%v, VerifyAssertion err=%v", subErr, asrErr)
			}
			if got.Subject != sub {
				t.Errorf("Subject = %q, VerifySubject = %q", got.Subject, sub)
			}
		})
	}
}
