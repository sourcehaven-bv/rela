package dataentry

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// installAccountConfig publishes a copy of the app's config with the given
// account block, as installNavStatusConfig does for navigation.
func installAccountConfig(app *App, acc *dataentryconfig.AccountConfig) {
	cur := app.State()
	next := *cur
	cfg := *cur.Cfg
	cfg.Account = acc
	next.Cfg = &cfg
	app.schema.Publish(&next)
}

// meAs calls GET /api/v1/_me with ctx and returns the raw body.
func meAs(ctx context.Context, t *testing.T, app *App) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/_me", http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()
	handleV1Me(app, rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Contains(t, rec.Header().Get("Cache-Control"), "no-store",
		"the response describes one principal, so no shared cache may hold it")
	return rec.Body.String()
}

func decodeMe(t *testing.T, body string) v1.MeResponse {
	t.Helper()
	var resp v1.MeResponse
	require.NoError(t, json.Unmarshal([]byte(body), &resp))
	return resp
}

// seedPersonTicket seeds the entity the tests use as the principal's person.
// The shared fixture has no person type; a ticket with a title and a `file`
// property is the same shape as far as _me is concerned.
func seedPersonTicket(app *App, props map[string]any) {
	p := map[string]any{"title": "Ada Lovelace"}
	maps.Copy(p, props)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: p})
}

func fullAccount() *dataentryconfig.AccountConfig {
	return &dataentryconfig.AccountConfig{
		SignOut:        "/pratique/auth/logout",
		Account:        "/pratique/admin/account",
		SwitchOrg:      "/pratique/auth/select-tenant",
		Admin:          &dataentryconfig.AccountAdminLink{URL: "/pratique/admin/", Role: "org-admin"},
		AvatarProperty: "screenshot",
	}
}

func TestMe_FullAssertion(t *testing.T) {
	app := newTestAppV1(t)
	seedPersonTicket(app, map[string]any{"screenshot": "attachments/TKT-001/screenshot/ada photo.png"})
	installAccountConfig(app, fullAccount())

	p := principal.VerifiedFrom("TKT-001", principal.ToolDataEntry, principal.Claims{
		OrgID: "o1", OrgSlug: "acme", OrgName: "Acme", Roles: []string{"editor", "org-admin"},
		Email: "a@example.com",
	})
	body := meAs(principal.With(context.Background(), p), t, app)

	assert.JSONEq(t, `{
		"user": "TKT-001",
		"email": "a@example.com",
		"org": {"id": "o1", "slug": "acme", "name": "Acme"},
		"roles": ["editor", "org-admin"],
		"person": {
			"type": "ticket", "id": "TKT-001", "title": "Ada Lovelace",
			"avatar": "/api/v1/tickets/TKT-001/_attachments/screenshot/ada%20photo.png"
		},
		"links": {
			"sign_out": "/pratique/auth/logout",
			"account": "/pratique/admin/account",
			"switch_org": "/pratique/auth/select-tenant",
			"admin": "/pratique/admin/"
		}
	}`, body)
}

// A proxy that sends only `sub`, with no account config and no matching
// entity, yields the user id and nothing else.
func TestMe_SubjectOnly(t *testing.T) {
	app := newTestAppV1(t)
	p := principal.VerifiedFrom("usr_1", principal.ToolDataEntry, principal.Claims{})
	body := meAs(principal.With(context.Background(), p), t, app)
	assert.JSONEq(t, `{"user": "usr_1"}`, body)
}

// RELA_DATAENTRY_USER stamps a plain principal naming an entity id. It
// carries no claims, but the person entity still resolves.
func TestMe_EnvUserNamingAnEntity(t *testing.T) {
	app := newTestAppV1(t)
	seedPersonTicket(app, nil)
	body := meAs(principalCtx("TKT-001"), t, app)
	assert.JSONEq(t, `{
		"user": "TKT-001",
		"person": {"type": "ticket", "id": "TKT-001", "title": "Ada Lovelace"}
	}`, body)
}

// The person is read through the ACL row gate: a principal who may not read
// its own entity gets no person at all, not a redacted one. The control case
// grants the read, so the omission is the gate's doing.
func TestMe_PersonHiddenByACLIsOmitted(t *testing.T) {
	for _, tc := range []struct {
		name       string
		readTypes  string
		wantPerson bool
	}{
		{"readable", "[ticket]", true},
		{"hidden", "[feature]", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, d := buildFieldPolicyApp(t, `
roles:
  viewer:
    read: `+tc.readTypes+`
assignments:
  TKT-001: viewer
`)
			seedPersonTicket(app, nil)
			resp := decodeMe(t, meAs(gateCtxFor(principalCtx("TKT-001"), t, d), t, app))
			assert.Equal(t, "TKT-001", resp.User)
			if tc.wantPerson {
				require.NotNil(t, resp.Person)
				assert.Equal(t, "Ada Lovelace", resp.Person.Title)
			} else {
				assert.Nil(t, resp.Person, "a hidden person entity must be omitted")
			}
		})
	}
}

// The avatar comes from the REDACTED entity: a `visible:` rule hiding the
// avatar property hides the avatar, while the person itself stays.
func TestMe_AvatarHiddenByFieldRedaction(t *testing.T) {
	app, d := buildFieldPolicyApp(t, `
roles:
  viewer:
    read: [ticket]
    visible:
      ticket:
        - field: title
assignments:
  TKT-001: viewer
`)
	seedPersonTicket(app, map[string]any{"screenshot": "attachments/TKT-001/screenshot/ada.png"})
	installAccountConfig(app, &dataentryconfig.AccountConfig{AvatarProperty: "screenshot"})

	resp := decodeMe(t, meAs(gateCtxFor(principalCtx("TKT-001"), t, d), t, app))
	require.NotNil(t, resp.Person)
	assert.Equal(t, "Ada Lovelace", resp.Person.Title)
	assert.Empty(t, resp.Person.Avatar, "a redacted avatar property must not produce an avatar")
}

func TestMe_Avatar(t *testing.T) {
	for _, tc := range []struct {
		name    string
		prop    string
		props   map[string]any
		want    string
		account bool
	}{
		{"file property", "screenshot",
			map[string]any{"screenshot": "attachments/TKT-001/screenshot/a.png"},
			"/api/v1/tickets/TKT-001/_attachments/screenshot/a.png", true},
		{"multi-file property takes the first", "docs",
			map[string]any{"docs": []any{"attachments/TKT-001/docs/b.png", "attachments/TKT-001/docs/c.png"}},
			"/api/v1/tickets/TKT-001/_attachments/docs/b.png", true},
		{"file property empty", "screenshot", map[string]any{"screenshot": ""}, "", true},
		{"property unset on the entity", "screenshot", nil, "", true},
		{"no avatar_property configured", "",
			map[string]any{"screenshot": "attachments/TKT-001/screenshot/a.png"}, "", true},
		{"no account block", "",
			map[string]any{"screenshot": "attachments/TKT-001/screenshot/a.png"}, "", false},
		{"https string", "status", map[string]any{"status": "https://img.example.com/a.png"},
			"https://img.example.com/a.png", true},
		{"root-relative string", "status", map[string]any{"status": "/static/a.png"}, "/static/a.png", true},
		{"javascript string dropped", "status", map[string]any{"status": "javascript:alert(1)"}, "", true},
		{"http string dropped", "status", map[string]any{"status": "http://img.example.com/a.png"}, "", true},
		{"protocol-relative string dropped", "status", map[string]any{"status": "//evil.example/a.png"}, "", true},
		{"relative string dropped", "status", map[string]any{"status": "a.png"}, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := newTestAppV1(t)
			seedPersonTicket(app, tc.props)
			if tc.account {
				installAccountConfig(app, &dataentryconfig.AccountConfig{AvatarProperty: tc.prop})
			}
			resp := decodeMe(t, meAs(principalCtx("TKT-001"), t, app))
			require.NotNil(t, resp.Person)
			assert.Equal(t, tc.want, resp.Person.Avatar)
		})
	}
}

// The admin link is a display filter on the asserted roles.
func TestMe_AdminLinkFilteredByRole(t *testing.T) {
	for _, tc := range []struct {
		name      string
		role      string
		roles     []string
		wantAdmin string
	}{
		{"principal holds the role", "org-admin", []string{"editor", "org-admin"}, "/admin/"},
		{"principal lacks the role", "org-admin", []string{"editor"}, ""},
		{"principal has no roles", "org-admin", nil, ""},
		{"no role configured", "", nil, "/admin/"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := newTestAppV1(t)
			installAccountConfig(app, &dataentryconfig.AccountConfig{
				SignOut: "/logout",
				Admin:   &dataentryconfig.AccountAdminLink{URL: "/admin/", Role: tc.role},
			})
			p := principal.VerifiedFrom("usr_1", principal.ToolDataEntry, principal.Claims{Roles: tc.roles})
			resp := decodeMe(t, meAs(principal.With(context.Background(), p), t, app))
			require.NotNil(t, resp.Links)
			assert.Equal(t, "/logout", resp.Links.SignOut)
			assert.Equal(t, tc.wantAdmin, resp.Links.Admin)
		})
	}
}

// A live config reload does not re-run validation, so the handler re-checks
// every link and drops an invalid one rather than serving it. With no valid
// link left, links is omitted.
func TestMeLinks_DropsInvalidLinks(t *testing.T) {
	got := meLinks(&dataentryconfig.AccountConfig{
		SignOut:   "javascript:alert(1)",
		Account:   "//evil.example/",
		SwitchOrg: "https://login.example.com/select",
	}, nil)
	require.NotNil(t, got)
	assert.Equal(t, v1.MeLinks{SwitchOrg: "https://login.example.com/select"}, *got)

	assert.Nil(t, meLinks(&dataentryconfig.AccountConfig{SignOut: "http://x"}, nil))
	assert.Nil(t, meLinks(&dataentryconfig.AccountConfig{AvatarProperty: "photo"}, nil))
	assert.Nil(t, meLinks(nil, nil))
}

// fakePeople is a personGetter returning a fixed result.
type fakePeople struct {
	e   *entity.Entity
	err error
}

func (f fakePeople) Get(context.Context, string, string) (*entity.Entity, bool, error) {
	return f.e, f.e != nil, f.err
}

// When acl.yaml names a user_entity_type, only an entity of that type is the
// person; a gate fault omits the person rather than failing the request.
func TestMePerson_TypeAndGateFault(t *testing.T) {
	app := newTestAppV1(t)
	e := &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "Ada"}}
	typeOf := func(context.Context, string) string { return "ticket" }
	base := meDeps{meta: app.Meta(), people: fakePeople{e: e}, typeOf: typeOf}

	require.NotNil(t, mePerson(context.Background(), "TKT-001", base))

	wrongType := base
	wrongType.userType = "feature"
	assert.Nil(t, mePerson(context.Background(), "TKT-001", wrongType))

	fault := base
	fault.people = fakePeople{err: errors.New("gate down")}
	assert.Nil(t, mePerson(context.Background(), "TKT-001", fault))

	missing := base
	missing.typeOf = func(context.Context, string) string { return "" }
	assert.Nil(t, mePerson(context.Background(), "TKT-001", missing))
}

func TestCheckAvatarUserType(t *testing.T) {
	app := newTestAppV1(t)
	policy := func(userType string) acl.ACL {
		return mustNewACL(t, &acl.Policy{UserEntityType: userType}, app.store)
	}
	for _, tc := range []struct {
		name    string
		acc     *dataentryconfig.AccountConfig
		aclImpl acl.ACL
		wantErr bool
	}{
		{"file property on the user type", &dataentryconfig.AccountConfig{AvatarProperty: "screenshot"},
			policy("ticket"), false},
		{"string property on the user type", &dataentryconfig.AccountConfig{AvatarProperty: "status"},
			policy("ticket"), false},
		{"property missing on the user type", &dataentryconfig.AccountConfig{AvatarProperty: "screenshot"},
			policy("feature"), true},
		{"no user type", &dataentryconfig.AccountConfig{AvatarProperty: "screenshot"}, policy(""), false},
		{"no ACL", &dataentryconfig.AccountConfig{AvatarProperty: "screenshot"}, acl.NopACL{}, false},
		{"no avatar property", &dataentryconfig.AccountConfig{}, policy("feature"), false},
		{"no account block", nil, policy("feature"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := checkAvatarUserType(tc.acc, tc.aclImpl, app.Meta())
			if tc.wantErr {
				assert.ErrorContains(t, err, "account.avatar_property")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// org_name travels from the verified assertion onto the Principal, and
// survives the principal_property re-stamp that _me reads after.
func TestJWTPrincipalResolver_CarriesOrgName(t *testing.T) {
	for _, tc := range []struct {
		name, orgName string
	}{
		{"present", "Acme Corp"},
		{"absent", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := stubVerifier{validToken: "t", subject: "usr_1", orgID: "org_a", orgName: tc.orgName}
			got := runResolver(t,
				ChainResolvers(JWTPrincipalResolver(v, assertionHeader)),
				map[string]string{assertionHeader: "t"})
			assert.Equal(t, tc.orgName, got.OrgName())
		})
	}
}

func TestPrincipalProperty_ReStampKeepsOrgName(t *testing.T) {
	d := buildLookupACL(t)
	var seen principal.Principal
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen = principal.From(r.Context())
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets", http.NoBody)
	req = req.WithContext(principal.With(req.Context(), principal.VerifiedFrom(
		"jvloothuis@sourcehaven.nl", principal.ToolDataEntry,
		principal.Claims{OrgID: "org_a", OrgName: "Acme Corp"})))
	attachACLRequest(next, d, true).ServeHTTP(httptest.NewRecorder(), req)

	require.Equal(t, "PERS-JV", seen.User)
	assert.Equal(t, "Acme Corp", seen.OrgName())
}
