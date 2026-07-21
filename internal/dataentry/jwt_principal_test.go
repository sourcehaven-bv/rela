package dataentry

import (
	"context"
	"errors"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// stubVerifier is an assertionVerifier that returns fixed claims for a known
// token and an error otherwise — lets the resolver's behavior be tested without
// real crypto (the crypto is covered in internal/jwtauth).
type stubVerifier struct {
	validToken string
	subject    string
	orgID      string
	orgSlug    string
	roles      []string
}

func (s stubVerifier) VerifyAssertion(_ context.Context, raw string) (AssertedIdentity, error) {
	if raw != s.validToken {
		return AssertedIdentity{}, errors.New("invalid")
	}
	return AssertedIdentity{
		Subject: s.subject,
		OrgID:   s.orgID,
		OrgSlug: s.orgSlug,
		Roles:   s.roles,
	}, nil
}

func TestJWTPrincipalResolver_VerifiedSubjectBecomesUser(t *testing.T) {
	v := stubVerifier{validToken: "good.jwt.token", subject: "usr_abc123"}
	got := runResolver(t,
		ChainResolvers(JWTPrincipalResolver(v, "X-Auth-Assertion")),
		map[string]string{"X-Auth-Assertion": "good.jwt.token"})
	if got.User != "usr_abc123" {
		t.Errorf("User = %q, want the verified subject", got.User)
	}
	// Tool is the ENTRY POINT (data-entry), not the IdP.
	if got.Tool != principal.ToolDataEntry {
		t.Errorf("Tool = %q, want %q", got.Tool, principal.ToolDataEntry)
	}
}

func TestJWTPrincipalResolver_StripsBearerPrefix(t *testing.T) {
	v := stubVerifier{validToken: "good.jwt.token", subject: "usr_abc123"}
	// The Bearer scheme is stripped case-insensitively (RFC 6750) with any run of
	// whitespace after it — so all these header values resolve to the same user.
	for _, header := range []string{
		"Bearer good.jwt.token",
		"bearer good.jwt.token",
		"BEARER good.jwt.token",
		"Bearer\tgood.jwt.token",
		"Bearer  good.jwt.token",
		"  Bearer good.jwt.token  ",
	} {
		got := runResolver(t,
			ChainResolvers(JWTPrincipalResolver(v, "Authorization")),
			map[string]string{"Authorization": header})
		if got.User != "usr_abc123" {
			t.Errorf("header %q: User = %q, want subject (Bearer should be stripped)", header, got.User)
		}
	}
}

// TestStripBearer covers the parser directly, including the "don't mangle a token
// that merely starts with 'bearer'" case.
func TestStripBearer(t *testing.T) {
	cases := map[string]string{
		"good.jwt.token":         "good.jwt.token",
		"Bearer good.jwt.token":  "good.jwt.token",
		"bearer\tgood.jwt.token": "good.jwt.token",
		"  raw.token  ":          "raw.token",
		"bearerish.token":        "bearerish.token", // no whitespace after 'bearer' → not a scheme
		"":                       "",
	}
	for in, want := range cases {
		if got := stripBearer(in); got != want {
			t.Errorf("stripBearer(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestJWTPrincipalResolver_InvalidTokenFallsThrough(t *testing.T) {
	v := stubVerifier{validToken: "good.jwt.token", subject: "usr_abc123"}
	// A forged/garbage token fails verification → zero Principal → chain falls
	// through to the default "unknown" (NOT a 401, and NOT the forged value).
	got := runResolver(t,
		ChainResolvers(JWTPrincipalResolver(v, "X-Auth-Assertion")),
		map[string]string{"X-Auth-Assertion": "forged.token"})
	if got.User != "unknown" {
		t.Errorf("User = %q, want 'unknown' (invalid token must not authenticate)", got.User)
	}
}

func TestJWTPrincipalResolver_MissingHeaderFallsThrough(t *testing.T) {
	v := stubVerifier{validToken: "good.jwt.token", subject: "usr_abc123"}
	got := runResolver(t,
		ChainResolvers(JWTPrincipalResolver(v, "X-Auth-Assertion")),
		nil)
	if got.User != "unknown" {
		t.Errorf("User = %q, want 'unknown'", got.User)
	}
}

func TestJWTPrincipalResolver_InertWhenDisabled(t *testing.T) {
	// nil verifier or empty header ⇒ inert resolver (matches the disabled-flag
	// shape of the other resolvers): every request falls through.
	for _, r := range []PrincipalResolver{
		JWTPrincipalResolver(nil, "X-Auth-Assertion"),
		JWTPrincipalResolver(stubVerifier{}, ""),
	} {
		got := runResolver(t, ChainResolvers(r),
			map[string]string{"X-Auth-Assertion": "good.jwt.token"})
		if got.User != "unknown" {
			t.Errorf("disabled resolver should fall through, got User=%q", got.User)
		}
	}
}

// TestJWTPrincipalResolver_ChainPrefersEnvOverride confirms the env escape hatch
// still wins over a verified JWT (the documented chain order: env → jwt → header).
func TestJWTPrincipalResolver_ChainPrefersEnvOverride(t *testing.T) {
	t.Setenv(envDataEntryUser, "dev-override")
	v := stubVerifier{validToken: "good.jwt.token", subject: "usr_abc123"}
	got := runResolver(t,
		ChainResolvers(EnvPrincipalResolver(), JWTPrincipalResolver(v, "X-Auth-Assertion")),
		map[string]string{"X-Auth-Assertion": "good.jwt.token"})
	if got.User != "dev-override" {
		t.Errorf("User = %q, want the env override to win", got.User)
	}
}
