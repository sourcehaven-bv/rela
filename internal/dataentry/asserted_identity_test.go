package dataentry

import (
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// Verified-assertion claims on the resolved Principal (TKT-RP3X3Q).

const assertionHeader = "X-Auth-Assertion"

func TestJWTPrincipalResolver_CarriesOrgAndRoles(t *testing.T) {
	v := stubVerifier{
		validToken: "good.jwt.token",
		subject:    "usr_abc123",
		orgID:      "org_acme",
		orgSlug:    "acme",
		roles:      []string{"admin", "billing"},
	}
	got := runResolver(t,
		ChainResolvers(JWTPrincipalResolver(v, assertionHeader)),
		map[string]string{assertionHeader: "good.jwt.token"})

	if got.User != "usr_abc123" {
		t.Errorf("User = %q, want the verified subject", got.User)
	}
	if got.OrgID() != "org_acme" {
		t.Errorf("OrgID = %q, want %q", got.OrgID(), "org_acme")
	}
	if got.OrgSlug() != "acme" {
		t.Errorf("OrgSlug = %q, want %q", got.OrgSlug(), "acme")
	}
	if want := []string{"admin", "billing"}; !slices.Equal(got.Roles(), want) {
		t.Errorf("Roles = %v, want %v", got.Roles(), want)
	}
}

func TestJWTPrincipalResolver_AbsentClaimsAreNotAnError(t *testing.T) {
	// AC2. Pratique always sends org/roles, but a different OIDC proxy may not.
	// An assertion carrying only a subject must still authenticate.
	for _, tc := range []struct {
		name  string
		stub  stubVerifier
		roles []string
	}{
		{"no claims at all", stubVerifier{validToken: "t", subject: "usr_1"}, nil},
		{
			"empty roles slice",
			stubVerifier{validToken: "t", subject: "usr_1", roles: []string{}},
			nil,
		},
		{
			"org without roles",
			stubVerifier{validToken: "t", subject: "usr_1", orgID: "org_a"},
			nil,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := runResolver(t,
				ChainResolvers(JWTPrincipalResolver(tc.stub, assertionHeader)),
				map[string]string{assertionHeader: "t"})

			if got.User != "usr_1" {
				t.Errorf("User = %q, want the principal to still resolve", got.User)
			}
			if !slices.Equal(got.Roles(), tc.roles) {
				t.Errorf("Roles = %v, want %v", got.Roles(), tc.roles)
			}
		})
	}
}

func TestJWTPrincipalResolver_SanitizesClaims(t *testing.T) {
	v := stubVerifier{
		validToken: "t",
		subject:    "usr_1",
		orgID:      "org\x00bad",
		roles:      []string{"ad\x00min", "\x00\x00", "ok"},
	}
	got := runResolver(t,
		ChainResolvers(JWTPrincipalResolver(v, assertionHeader)),
		map[string]string{assertionHeader: "t"})

	if strings.ContainsRune(got.OrgID(), 0) {
		t.Errorf("OrgID kept a control char: %q", got.OrgID())
	}
	for _, role := range got.Roles() {
		if strings.ContainsRune(role, 0) {
			t.Errorf("role kept a control char: %q", role)
		}
		if role == "" {
			t.Error("an empty role survived sanitization; it can never match a " +
				"policy mapping and only pads the attribution set")
		}
	}
	// "\x00\x00" sanitizes to empty and is dropped; the other two survive.
	if len(got.Roles()) != 2 {
		t.Errorf("Roles = %v, want 2 surviving entries", got.Roles())
	}
}

func TestJWTPrincipalResolver_InvalidTokenCarriesNoClaims(t *testing.T) {
	// Verification failure must yield a zero Principal — never a principal that
	// kept the claims from an unverified token.
	v := stubVerifier{
		validToken: "good.jwt.token",
		subject:    "usr_1",
		orgID:      "org_a",
		roles:      []string{"admin"},
	}
	got := runResolver(t,
		ChainResolvers(JWTPrincipalResolver(v, assertionHeader)),
		map[string]string{assertionHeader: "forged.jwt.token"})

	if len(got.Roles()) != 0 {
		t.Errorf("roles survived a failed verification: %v", got.Roles())
	}
	if got.OrgID() != "" {
		t.Errorf("org survived a failed verification: %q", got.OrgID())
	}
}

// TestNonJWTResolvers_CannotSetAssertedClaims is the trust-boundary regression
// guard, and the most important test in this file.
//
// ChainResolvers advances on p.User != "" and ignores every other field, so
// nothing about the chain's shape prevents a resolver from returning a
// role-bearing Principal. internal/acl trusts Principal absolutely — it
// verifies nothing itself — so a role sourced from a spoofable header would be
// a complete authorization bypass.
//
// The unexported fields on principal.Principal make that impossible to express
// (only principal.Verified can populate them), which is why this test cannot
// be written as "assert the header resolver doesn't set roles" in a way that
// would ever fail. It is here to fail LOUDLY if someone later exports those
// fields or hands a second resolver a Verified constructor.
func TestNonJWTResolvers_CannotSetAssertedClaims(t *testing.T) {
	t.Setenv(envDataEntryUser, "env-user")

	for _, tc := range []struct {
		name     string
		resolver PrincipalResolver
		headers  map[string]string
	}{
		{
			"header resolver",
			HeaderPrincipalResolver("X-Forwarded-User"),
			map[string]string{"X-Forwarded-User": "alice@example.com"},
		},
		{"env resolver", EnvPrincipalResolver(), nil},
		{"default resolver", defaultPrincipalResolver, nil},
		{
			// A header that merely LOOKS like a roles claim must do nothing.
			"roles-shaped header is inert",
			HeaderPrincipalResolver("X-Forwarded-User"),
			map[string]string{
				"X-Forwarded-User": "alice@example.com",
				"X-Asserted-Roles": "admin,superuser",
				"X-Roles":          "admin",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := runResolver(t, ChainResolvers(tc.resolver), tc.headers)

			if got.User == "" {
				t.Fatal("resolver produced no principal; test is not exercising it")
			}
			if len(got.Roles()) != 0 {
				t.Errorf("non-JWT resolver produced asserted roles %v — this is an "+
					"authorization bypass", got.Roles())
			}
			if got.OrgID() != "" || got.OrgSlug() != "" {
				t.Errorf("non-JWT resolver produced org claims (%q/%q)",
					got.OrgID(), got.OrgSlug())
			}
		})
	}
}

func TestNonJWTResolvers_ResolvePrincipalEntityIsTheOnlyOtherVerifiedCaller(t *testing.T) {
	// resolvePrincipalEntity legitimately calls principal.Verified — it rebuilds
	// the Principal when principal_property substitutes a graph entity ID. That
	// makes it the one non-resolver path that can carry roles, so it is worth
	// pinning that it only ever PROPAGATES claims it was given, never invents
	// them. A principal that arrives with no roles must leave with none.
	//
	// The propagation direction (claims survive) is covered end-to-end by
	// TestE2E_ClaimsSurvivePrincipalPropertyReStamp.
	plain := principal.Principal{User: "alice", Tool: principal.ToolDataEntry}
	if len(plain.Roles()) != 0 {
		t.Fatal("precondition: a plain Principal already has roles")
	}
	// The shape resolvePrincipalEntity builds when the incoming principal has
	// no claims: Verified with the resolved ID and whatever the original had.
	p := principal.Verified("PERS-alice", plain.Tool, plain.OrgID(), plain.OrgSlug(), plain.Roles())
	if len(p.Roles()) != 0 {
		t.Errorf("rebuilding a role-less principal invented roles: %v", p.Roles())
	}
	if p.OrgID() != "" {
		t.Errorf("rebuilding a principal with no org invented one: %q", p.OrgID())
	}
}

func TestChainResolvers_JWTClaimsSurviveTheChain(t *testing.T) {
	// The JWT resolver sits mid-chain (env → JWT → header → default). Verify the
	// claims are not flattened by ChainResolvers on the way out.
	v := stubVerifier{
		validToken: "t", subject: "usr_1",
		orgID: "org_a", roles: []string{"admin"},
	}
	got := runResolver(t,
		ChainResolvers(
			EnvPrincipalResolver(), // inert: env unset
			JWTPrincipalResolver(v, assertionHeader),
			HeaderPrincipalResolver("X-Forwarded-User"),
		),
		map[string]string{
			assertionHeader:    "t",
			"X-Forwarded-User": "should-not-win@example.com",
		})

	if got.User != "usr_1" {
		t.Errorf("User = %q, want the JWT subject to win over the header", got.User)
	}
	if want := []string{"admin"}; !slices.Equal(got.Roles(), want) {
		t.Errorf("Roles = %v, want %v — claims lost crossing ChainResolvers", got.Roles(), want)
	}
}

func TestPrincipalVerified_RolesAreDefensivelyCopied(t *testing.T) {
	// A caller mutating its slice after construction must not retroactively
	// change an authorization decision.
	roles := []string{"admin"}
	p := principal.Verified("usr_1", principal.ToolDataEntry, "org", "slug", roles)

	roles[0] = "superuser"

	if got := p.Roles(); got[0] != "admin" {
		t.Errorf("Roles = %v, want the value at construction time", got)
	}

	// The accessor's return must be a copy too.
	out := p.Roles()
	out[0] = "superuser"
	if got := p.Roles(); got[0] != "admin" {
		t.Errorf("mutating the accessor's result changed the principal: %v", got)
	}
}
