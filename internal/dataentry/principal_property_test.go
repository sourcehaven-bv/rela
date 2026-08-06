package dataentry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

const principalPropertyPolicy = `
user_entity_type: persoon
principal_property: email
membership_relation: heeft_rol
assignments:
  ROLE-MD: md
roles:
  everyone:
    read: ["*"]
  md:
    read: ["*"]
    update: ["*"]
`

// buildLookupACL creates a memstore with a persoon carrying an email and a
// heeft_rol edge to ROLE-MD, plus a Declarative wired with the
// principal_property lookup.
func buildLookupACL(t *testing.T) *acl.Declarative {
	t.Helper()
	ctx := context.Background()
	ms := memstore.New()
	jv := entity.New("PERS-JV", "persoon")
	jv.SetString("email", "jvloothuis@sourcehaven.nl")
	if err := ms.CreateEntity(ctx, jv); err != nil {
		t.Fatalf("create PERS-JV: %v", err)
	}
	if err := ms.CreateEntity(ctx, entity.New("ROLE-MD", "rol")); err != nil {
		t.Fatalf("create ROLE-MD: %v", err)
	}
	if _, err := ms.CreateRelation(ctx, "PERS-JV", "heeft_rol", "ROLE-MD", nil); err != nil {
		t.Fatalf("create heeft_rol: %v", err)
	}

	p, err := acl.LoadPolicyBytes([]byte(principalPropertyPolicy))
	if err != nil {
		t.Fatalf("LoadPolicyBytes: %v", err)
	}
	d, err := acl.NewDeclarative(p, acl.NewStoreGraph(ms), ms,
		acl.WithPrincipalLookup(acl.NewStorePrincipalLookup(ms)))
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	return d
}

// TestPrincipalProperty_MiddlewareReStampsCtx verifies that the data-entry
// ACL middleware resolves the raw principal (email) to the user entity ID
// and re-stamps ctx so the resolved principal — carrying both the entity
// ID (User) and the raw header (RawUser) — is what downstream handlers and
// the audit writer see via principal.From(ctx).
func TestPrincipalProperty_MiddlewareReStampsCtx(t *testing.T) {
	d := buildLookupACL(t)

	var seen principal.Principal
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen = principal.From(r.Context())
	})
	handler := attachACLRequest(next, d, false)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets", http.NoBody)
	req = req.WithContext(principal.With(req.Context(),
		principal.Principal{User: "jvloothuis@sourcehaven.nl", Tool: principal.ToolDataEntry}))
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if seen.User != "PERS-JV" {
		t.Errorf("resolved User = %q, want PERS-JV", seen.User)
	}
	if seen.RawUser != "jvloothuis@sourcehaven.nl" {
		t.Errorf("RawUser = %q, want the original email", seen.RawUser)
	}
	if seen.Tool != principal.ToolDataEntry {
		t.Errorf("Tool = %q, want data-entry", seen.Tool)
	}
}

// TestPrincipalProperty_MiddlewareKeepsRawOnNoMatch verifies that a
// principal with no matching persoon is left untouched (no RawUser, User
// unchanged) — the fail-open-to-raw fallback.
func TestPrincipalProperty_MiddlewareKeepsRawOnNoMatch(t *testing.T) {
	d := buildLookupACL(t)

	var seen principal.Principal
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen = principal.From(r.Context())
	})
	handler := attachACLRequest(next, d, false)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets", http.NoBody)
	req = req.WithContext(principal.With(req.Context(),
		principal.Principal{User: "nobody@sourcehaven.nl", Tool: principal.ToolDataEntry}))
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if seen.User != "nobody@sourcehaven.nl" {
		t.Errorf("User = %q, want the raw principal unchanged", seen.User)
	}
	if seen.RawUser != "" {
		t.Errorf("RawUser = %q, want empty when no substitution happened", seen.RawUser)
	}
}
