package dataentry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
)

// Inline entity creation from a relation field (TKT-OMUD56) is offered for a
// target type when BOTH conditions hold: the principal may create that type,
// and a create form resolves for it. The server answers both at once by
// listing the resolved form id per eligible type on the sidebar payload, so
// presence in `inline_create` IS the affordance.
//
// These tests pin that contract at the handler boundary. The client-side half
// (the depth cap, the widget rendering) lives in the Vitest suites.

// installInlineCreateConfig publishes forms for two entity types plus one type
// with no form at all, so every arm of the eligibility rule is observable.
func installInlineCreateConfig(app *App, forms map[string]dataentryconfig.Form) {
	cur := app.State()
	next := *cur
	cfg := *cur.Cfg
	cfg.Forms = forms
	next.Cfg = &cfg
	app.schema.Publish(&next)
}

func inlineCreateMap(ctx context.Context, t *testing.T, app *App) map[string]string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/_sidebar", http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()
	app.views.handleV1Sidebar(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET _sidebar: %d %s", rec.Code, rec.Body)
	}
	var resp v1.SidebarResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode sidebar: %v\nbody: %s", err, rec.Body)
	}
	return resp.InlineCreate
}

// TestInlineCreate_RequiresAForm covers AC1/AC3: a type is offered only when a
// form resolves for it, and the offered value is that form's id.
func TestInlineCreate_RequiresAForm(t *testing.T) {
	app := newTestAppV1(t)
	installInlineCreateConfig(app, map[string]dataentryconfig.Form{
		"create_ticket": {EntityType: "ticket"},
	})

	got := inlineCreateMap(t.Context(), t, app)

	if got["ticket"] != "create_ticket" {
		t.Errorf("ticket: got form %q, want create_ticket", got["ticket"])
	}
	// A type the metamodel declares but no form targets must be absent —
	// not present-with-empty-string, which the SPA would read as offerable.
	if _, ok := got["feature"]; ok {
		t.Errorf("feature has no form but was offered: %v", got)
	}
}

// TestInlineCreate_EditOnlyFormIsOffered pins RR-KGCF61. createFormForType
// deliberately falls back to an edit-mode form (it works for creation when no
// entity id is supplied), so an edit-only type IS offered. The boundary that
// hides the affordance is "no form at all", not "no create-mode form" — an
// earlier draft of this ticket asserted the opposite and would have failed.
func TestInlineCreate_EditOnlyFormIsOffered(t *testing.T) {
	app := newTestAppV1(t)
	installInlineCreateConfig(app, map[string]dataentryconfig.Form{
		"edit_ticket": {EntityType: "ticket", Mode: "edit"},
	})

	if got := inlineCreateMap(t.Context(), t, app); got["ticket"] != "edit_ticket" {
		t.Errorf("edit-only type: got %q, want edit_ticket as the fallback form", got["ticket"])
	}
}

// TestInlineCreate_GatedByCreatePermission covers AC2: a form is not enough,
// the principal must also hold `create` on the type.
//
// It also pins the payload as principal-DEPENDENT, which is the reason this
// rides on the sidebar rather than `_config` (pinned principal-independent by
// TestNavPermission_ConfigUnfiltered) or `_schema` (a pure metamodel
// projection).
func TestInlineCreate_GatedByCreatePermission(t *testing.T) {
	app := newTestAppV1(t)
	installInlineCreateConfig(app, map[string]dataentryconfig.Form{
		"create_ticket": {EntityType: "ticket"},
	})
	d := mustNewACL(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"author": {Read: []string{"ticket"}, Create: []string{"ticket"}},
			"reader": {Read: []string{"ticket"}},
		},
		Assignments: map[string]string{"alice": "author", "bob": "reader"},
	}, app.store)
	app.acl = d

	alice := inlineCreateMap(gateCtxFor(aliceCtx(), t, d), t, app)
	if alice["ticket"] != "create_ticket" {
		t.Errorf("alice may create tickets but was not offered: %v", alice)
	}

	bob := inlineCreateMap(gateCtxFor(principalCtx("bob"), t, d), t, app)
	if _, ok := bob["ticket"]; ok {
		t.Errorf("bob may not create tickets but was offered: %v", bob)
	}
}

// TestInlineCreate_CreateWithoutReadIsOffered guards the submitter role.
//
// acl.Policy documents that create implies NO read: a role may create a type it
// cannot read. Deriving the offer from read visibility would therefore silently
// deny inline create to exactly the principals it is most useful for, so this
// asserts the offer survives a create-only grant.
func TestInlineCreate_CreateWithoutReadIsOffered(t *testing.T) {
	app := newTestAppV1(t)
	installInlineCreateConfig(app, map[string]dataentryconfig.Form{
		"create_ticket": {EntityType: "ticket"},
	})
	d := mustNewACL(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"submitter": {Create: []string{"ticket"}},
		},
		Assignments: map[string]string{"alice": "submitter"},
	}, app.store)
	app.acl = d

	got := inlineCreateMap(gateCtxFor(aliceCtx(), t, d), t, app)
	if got["ticket"] != "create_ticket" {
		t.Errorf("create-without-read principal was not offered inline create: %v", got)
	}
}

// TestInlineCreate_OmittedWhenEmpty keeps the field off the wire entirely when
// nothing is offerable, rather than serializing `"inline_create": {}`.
func TestInlineCreate_OmittedWhenEmpty(t *testing.T) {
	app := newTestAppV1(t)
	installInlineCreateConfig(app, map[string]dataentryconfig.Form{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_sidebar", http.NoBody).WithContext(t.Context())
	rec := httptest.NewRecorder()
	app.views.handleV1Sidebar(rec, req)

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode sidebar: %v", err)
	}
	if _, ok := raw["inline_create"]; ok {
		t.Errorf("inline_create should be omitted when empty; body=%s", rec.Body)
	}
}
