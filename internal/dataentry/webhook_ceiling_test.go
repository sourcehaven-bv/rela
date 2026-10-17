package dataentry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/aclmap"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// A webhook delivery runs with principal_type webhook, so an acl.yaml
// baseline with `applies_to: [webhook]` caps it, including the automations
// its write fires (TKT-J3RDJ4).

// hookCeilingMeta: every new incident gets a note from a declarative
// create_entity action and a memo from an inline Lua action. Both run in the
// delivery's request. The Lua action writes as the hook, through the ACL; the
// declarative action is a system write that no caller's ACL gates.
const hookCeilingMeta = `
entities:
  incident:
    label: Incident
    id_prefix: INC
    properties:
      host: { type: string }
  note:
    label: Note
    id_prefix: NOTE
    properties:
      title: { type: string }
  memo:
    label: Memo
    id_prefix: MEMO
    properties:
      title: { type: string }
relations:
  has-note:
    from: [incident]
    to: [note]
automations:
  - name: note-on-incident
    on:
      entity: incident
      created: true
    do:
      - create_entity:
          type: note
          properties:
            title: "note for {{new.host}}"
          relation: has-note
      - lua: |
          rela.create_entity("memo", {title = "memo for " .. entity.properties.host})
`

// hookRole grants the hook incidents and notes.
var hookRole = acl.RoleDef{
	Read:   []string{"incident", "note", "memo"},
	Create: []string{"incident", "note", "memo"},
	Update: []string{"incident", "note", "memo"},
}

// incidentOnly is the baseline an operator adds to cap every hook.
var incidentOnly = acl.ClientBaseline{
	AppliesTo: []string{principal.TypeWebhook},
	Restriction: acl.Restriction{
		Read: []string{"incident"}, Create: []string{"incident"}, Update: []string{"incident"},
	},
}

var alertHook = map[string]dataentryconfig.Webhook{
	"alert": {
		Find:            &dataentryconfig.WebhookFind{Type: "incident", Match: []string{"host"}},
		CreateIfMissing: &dataentryconfig.WebhookCreate{},
	},
}

// hookACLApp builds an App whose writes, automations and reads all go
// through the policy, with the alert hook configured.
func hookACLApp(t *testing.T, baselines map[string]acl.ClientBaseline) (*App, store.Store, *audit.Memory) {
	t.Helper()
	meta, err := metamodel.Parse([]byte(hookCeilingMeta))
	if err != nil {
		t.Fatalf("parse metamodel: %v", err)
	}
	fs := storage.NewMemFS()
	paths := &project.Context{Root: "/project", CacheDir: "/project/.rela"}
	if mkErr := fs.MkdirAll(paths.CacheDir, 0o755); mkErr != nil {
		t.Fatal(mkErr)
	}
	st := appbuildtest.New(meta, appbuildtest.WithFS(fs, paths)).Store()
	d := mustNewACL(t, &acl.Policy{
		Roles:           map[string]acl.RoleDef{"icinga": hookRole},
		Assignments:     map[string]string{"system:webhook:alert": "icinga"},
		ClientBaselines: baselines,
	}, st)
	sink := audit.NewMemory()
	svc := appbuildtest.New(meta, appbuildtest.WithFS(fs, paths), appbuildtest.WithStore(st),
		appbuildtest.WithDeclarative(d), appbuildtest.WithAudit(sink))
	cfg := &Config{Webhooks: alertHook}
	app := newAppFromParts(cfg, nil, newFixture())
	rebindApp(app, fs, paths, svc)
	setTestACL(t, app, d)
	resolver, err := ResolverFromProfile("", meta, st, d)
	if err != nil {
		t.Fatalf("ResolverFromProfile: %v", err)
	}
	setTestResolver(t, app, resolver)
	app.schema.Publish(&Schema{Cfg: cfg, Meta: meta})
	if err := app.SetSecurityConfig(SecurityConfig{BindAddress: "127.0.0.1:8080"}); err != nil {
		t.Fatalf("SetSecurityConfig: %v", err)
	}
	return app, st, sink
}

func countType(t *testing.T, st store.Store, typ string) int {
	t.Helper()
	n := 0
	for _, err := range st.ListEntities(context.Background(),
		store.EntityQuery{Type: typ, Faces: store.InWorld(store.TrivialScope())}) {
		if err != nil {
			t.Fatalf("list %s: %v", typ, err)
		}
		n++
	}
	return n
}

// TestWebhookCeiling_NoBaselineBehavesAsBefore pins AC2: a policy without a
// webhook baseline leaves the hook with its role's full access, so the
// automation's note is written.
func TestWebhookCeiling_NoBaselineBehavesAsBefore(t *testing.T) {
	app, st, _ := hookACLApp(t, nil)
	if rec := postHook(t, app, "alert", `{"host":"web-1"}`); rec.Code >= 300 {
		t.Fatalf("delivery = %d %s", rec.Code, rec.Body.String())
	}
	if n := countType(t, st, "incident"); n != 1 {
		t.Errorf("incidents = %d, want 1", n)
	}
	for _, typ := range []string{"note", "memo"} {
		if n := countType(t, st, typ); n != 1 {
			t.Errorf("%ss = %d, want 1 (the automation runs uncapped)", typ, n)
		}
	}
}

// TestWebhookCeiling_CapsTheAutomation pins AC4: the Lua action fired by the
// hook's write runs as the hook and meets its ceiling. The hook's own write
// stands; the refused write does not fail the delivery. The declarative
// create_entity action is not capped: declarative actions are system writes
// for every caller, not only for hooks.
func TestWebhookCeiling_CapsTheAutomation(t *testing.T) {
	app, st, sink := hookACLApp(t, map[string]acl.ClientBaseline{"hooks": incidentOnly})
	if rec := postHook(t, app, "alert", `{"host":"web-1"}`); rec.Code >= 300 {
		t.Fatalf("delivery = %d %s", rec.Code, rec.Body.String())
	}
	if n := countType(t, st, "incident"); n != 1 {
		t.Errorf("incidents = %d, want 1", n)
	}
	if n := countType(t, st, "memo"); n != 0 {
		t.Errorf("memos = %d, want 0: the ceiling must cap the Lua action", n)
	}
	if n := countType(t, st, "note"); n != 1 {
		t.Errorf("notes = %d, want 1: a declarative action is a system write", n)
	}

	// A second delivery for the same host finds the incident: the ceiling
	// keeps read on incident, so the find still matches.
	if rec := postHook(t, app, "alert", `{"host":"web-1"}`); rec.Code >= 300 {
		t.Fatalf("second delivery = %d %s", rec.Code, rec.Body.String())
	}
	if n := countType(t, st, "incident"); n != 1 {
		t.Errorf("incidents after a repeat delivery = %d, want 1", n)
	}

	// AC1: the audit record names the hook and its principal type.
	var seen bool
	for _, r := range sink.Records() {
		if r.Principal.User == "system:webhook:alert" {
			seen = true
			if got := r.Principal.PrincipalType(); got != principal.TypeWebhook {
				t.Errorf("audited principal type = %q, want %q", got, principal.TypeWebhook)
			}
		}
	}
	if !seen {
		t.Error("no audit record for system:webhook:alert")
	}
}

// TestWebhookCeiling_RefusesWriteOutsideBaseline pins AC3: a hook whose own
// write falls outside the baseline fails and writes nothing.
func TestWebhookCeiling_RefusesWriteOutsideBaseline(t *testing.T) {
	app, st, _ := hookACLApp(t, map[string]acl.ClientBaseline{"hooks": {
		AppliesTo:   []string{principal.TypeWebhook},
		Restriction: acl.Restriction{Read: []string{"incident"}, Create: []string{"note"}},
	}})
	if rec := postHook(t, app, "alert", `{"host":"web-1"}`); rec.Code < 400 {
		t.Fatalf("delivery = %d, want a failure: create incident is outside the baseline", rec.Code)
	}
	if n := countType(t, st, "incident"); n != 0 {
		t.Errorf("incidents = %d, want 0", n)
	}
}

// TestWebhookCeiling_UnreadableFindTypeDuplicates documents the hazard the
// `acl map --webhook` check exists for: a baseline that removes read on the
// find type makes every delivery miss and create again.
func TestWebhookCeiling_UnreadableFindTypeDuplicates(t *testing.T) {
	app, st, _ := hookACLApp(t, map[string]acl.ClientBaseline{"hooks": {
		AppliesTo:   []string{principal.TypeWebhook},
		Restriction: acl.Restriction{Read: []string{"note"}, Create: []string{"incident"}},
	}})
	for range 2 {
		if rec := postHook(t, app, "alert", `{"host":"web-1"}`); rec.Code >= 300 {
			t.Fatalf("delivery = %d %s", rec.Code, rec.Body.String())
		}
	}
	if n := countType(t, st, "incident"); n != 2 {
		t.Errorf("incidents = %d, want 2: the find cannot see what it created", n)
	}
}

// TestWebhookRoute_ReadsAsTheHook pins that the delivery's find reads through
// a gate bound to the hook principal. A Request opened for another principal
// before the route must not survive into the delivery: it belongs to a
// principal with no grants, so a find that reused it would never match. A
// missing gate denies every read under the policy, with the same effect.
// Either way every delivery would create a new incident.
func TestWebhookRoute_ReadsAsTheHook(t *testing.T) {
	app, st, _ := hookACLApp(t, nil)
	d, ok := app.acl.(*acl.Declarative)
	if !ok {
		t.Fatalf("app ACL is %T, want *acl.Declarative", app.acl)
	}
	stale, err := acl.OpenRequest(t.Context(), d, principal.Principal{User: "nobody", Tool: principal.ToolDataEntry})
	if err != nil {
		t.Fatal(err)
	}
	router := app.NewRouter()
	withStale := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		router.ServeHTTP(w, r.WithContext(acl.WithRequest(r.Context(), stale)))
	})
	for range 2 {
		req := httptest.NewRequest(http.MethodPost, "/hooks/alert", strings.NewReader(`{"host":"web-1"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Host = "127.0.0.1:8080"
		rec := httptest.NewRecorder()
		withStale.ServeHTTP(rec, req)
		if rec.Code >= 300 {
			t.Fatalf("delivery = %d %s", rec.Code, rec.Body.String())
		}
	}
	if n := countType(t, st, "incident"); n != 1 {
		t.Errorf("incidents = %d, want 1: the find ran under a stale Request", n)
	}
}

// TestWebhookCeiling_MapAgreesWithRuntime runs one policy through both the
// `rela acl map --webhook` check and real deliveries. The check must say OK
// exactly when the hook works: both deliveries succeed and the second finds
// the incident the first created.
func TestWebhookCeiling_MapAgreesWithRuntime(t *testing.T) {
	cases := []struct {
		name     string
		baseline *acl.ClientBaseline
	}{
		{"no baseline", nil},
		{"incident baseline", &incidentOnly},
		{"create outside the baseline", &acl.ClientBaseline{
			AppliesTo:   []string{principal.TypeWebhook},
			Restriction: acl.Restriction{Read: []string{"incident"}, Create: []string{"note"}},
		}},
		{"find type unreadable", &acl.ClientBaseline{
			AppliesTo:   []string{principal.TypeWebhook},
			Restriction: acl.Restriction{Read: []string{"note"}, Create: []string{"incident"}},
		}},
		{"match property redacted", &acl.ClientBaseline{
			AppliesTo: []string{principal.TypeWebhook},
			Restriction: acl.Restriction{
				Read: []string{"incident"}, Create: []string{"incident"},
				Redact: map[string][]string{"incident": {"host"}},
			},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var baselines map[string]acl.ClientBaseline
			if tc.baseline != nil {
				baselines = map[string]acl.ClientBaseline{"hooks": *tc.baseline}
			}
			app, st, _ := hookACLApp(t, baselines)
			d, ok := app.acl.(*acl.Declarative)
			if !ok {
				t.Fatalf("app ACL is %T, want *acl.Declarative", app.acl)
			}
			eng, err := aclmap.New(st, d, func(string) []entity.Face { return nil })
			if err != nil {
				t.Fatalf("aclmap.New: %v", err)
			}
			hook := alertHook["alert"]
			res, err := eng.MapWebhook(context.Background(), "alert", &aclmap.WebhookMapping{
				FindType: hook.Find.Type, Match: hook.Find.Match, CreateType: hook.Find.Type,
			}, "", "", []string{"incident", "note", "memo"}, aclmap.Extras{})
			if err != nil {
				t.Fatalf("MapWebhook: %v", err)
			}

			works := true
			for range 2 {
				if rec := postHook(t, app, "alert", `{"host":"web-1"}`); rec.Code >= 300 {
					works = false
				}
			}
			if countType(t, st, "incident") != 1 {
				works = false
			}
			if got := res.WebhookCheck.OK(); got != works {
				t.Errorf("map check OK = %v, runtime works = %v; needs %+v",
					got, works, res.WebhookCheck.Needs)
			}
		})
	}
}
