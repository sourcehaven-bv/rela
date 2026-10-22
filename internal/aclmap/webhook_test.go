package aclmap_test

import (
	"context"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/aclmap"
)

// `rela acl map --webhook` (TKT-J3RDJ4). The hook principal is the one the
// data-entry handler stamps, so the map must apply the `applies_to: [webhook]`
// ceiling without the caller naming the type, and must drop the everyone
// grants that ceiling removes.

// hookPolicy grants the Icinga hook more than it needs, plus an everyone
// read on project: the uncapped starting point of a real deployment.
const hookPolicy = `
roles:
  everyone: { read: [project] }
  icinga-webhook:
    read: [incident, ticket]
    create: [incident]
    update: [incident, ticket]
    delete: [incident]
    permissions: [comment.add]
assignments:
  system:webhook:icinga-alert: icinga-webhook
`

// hookCeiling is the baseline an operator adds to cap every hook.
const hookCeiling = `
client_baselines:
  hooks:
    applies_to: [webhook]
    read: [incident]
    create: [incident]
    update: [incident]
    deny_delete: ["*"]
    deny_permissions: ["*"]
`

var hookTypes = []string{"incident", "person", "project", "ticket"}

func hookExtras() aclmap.Extras {
	return aclmap.Extras{
		Permissions: []string{"comment.add", "*", "comment.add", "history.read"},
		Relations:   []aclmap.RelationType{{Name: "affects", From: []string{"incident", "ticket"}}},
	}
}

func hookWorld(t *testing.T, policy string) *world {
	t.Helper()
	return buildWorld(t, policy,
		[]ent{{"INC-1", "incident"}, {"TKT-1", "ticket"}, {"PROJ", "project"}},
		nil,
	)
}

func mapHook(t *testing.T, w *world, mapping *aclmap.WebhookMapping) *aclmap.MapPrincipalResult {
	t.Helper()
	res, err := w.eng.MapWebhook(context.Background(), "icinga-alert", mapping, "", "", hookTypes, hookExtras())
	if err != nil {
		t.Fatalf("MapWebhook: %v", err)
	}
	return res
}

// typeVerbs returns the type-wide verbs of typ, sorted; nil when absent.
func typeVerbs(res *aclmap.MapPrincipalResult, typ string) []string {
	ta := typeOf(res, typ)
	if ta == nil {
		return nil
	}
	var out []string
	for v := range ta.Baseline {
		out = append(out, v)
	}
	slices.Sort(out)
	return out
}

func TestMapWebhook_WithoutBaselineIsUncapped(t *testing.T) {
	t.Parallel()
	res := mapHook(t, hookWorld(t, hookPolicy), nil)

	if res.Principal != "system:webhook:icinga-alert" || res.Webhook != "icinga-alert" || res.As != "webhook" {
		t.Errorf("identity = %q/%q/%q, want system:webhook:icinga-alert/icinga-alert/webhook",
			res.Principal, res.Webhook, res.As)
	}
	if res.Baseline != "" {
		t.Errorf("Baseline = %q, want empty: no baseline lists webhook", res.Baseline)
	}
	for typ, want := range map[string][]string{
		"incident": {"create", "delete", "read", "update"},
		"ticket":   {"read", "update"},
		"project":  {"read"},
	} {
		if got := typeVerbs(res, typ); !slices.Equal(got, want) {
			t.Errorf("%s verbs = %v, want %v", typ, got, want)
		}
	}
	if !slices.Equal(res.Permissions, []string{"comment.add"}) {
		t.Errorf("Permissions = %v, want [comment.add]", res.Permissions)
	}
	if len(res.Relations) != 1 || !slices.Equal(res.Relations[0].Verbs["update"], []string{"incident", "ticket"}) {
		t.Errorf("Relations = %+v, want affects update from incident and ticket", res.Relations)
	}
}

// TestMapWebhook_CeilingRemovesEveryoneAndRoleGrants is the headline: under
// the hook baseline the map shows exactly what the runtime allows. Before
// TKT-J3RDJ4 the everyone read on project was reported regardless of the
// ceiling, so an expectations file would have attested a read the runtime
// refuses.
func TestMapWebhook_CeilingRemovesEveryoneAndRoleGrants(t *testing.T) {
	t.Parallel()
	res := mapHook(t, hookWorld(t, hookPolicy+hookCeiling), nil)

	if res.Baseline != "hooks" {
		t.Errorf("Baseline = %q, want hooks", res.Baseline)
	}
	if got := typeVerbs(res, "incident"); !slices.Equal(got, []string{"create", "read", "update"}) {
		t.Errorf("incident verbs = %v, want [create read update]", got)
	}
	for _, typ := range []string{"project", "ticket"} {
		if ta := typeOf(res, typ); ta != nil {
			t.Errorf("%s still reported under the ceiling: %+v", typ, ta)
		}
	}
	if len(res.Permissions) != 0 {
		t.Errorf("Permissions = %v, want none (deny_permissions: [*])", res.Permissions)
	}
	if len(res.Relations) != 1 {
		t.Fatalf("Relations = %+v, want only affects", res.Relations)
	}
	verbs := res.Relations[0].Verbs
	if !slices.Equal(verbs["create"], []string{"incident"}) || !slices.Equal(verbs["update"], []string{"incident"}) {
		t.Errorf("affects verbs = %v, want create and update from incident", verbs)
	}
	if _, ok := verbs["delete"]; ok {
		t.Errorf("affects delete reported despite deny_delete: %v", verbs)
	}
}

// TestMapPrincipal_WebhookNameGetsTheCeiling pins that mapping the hook by its
// name, without --webhook, reports the same capped access: the type is
// derived from the name, so no report can show the hook uncapped.
func TestMapPrincipal_WebhookNameGetsTheCeiling(t *testing.T) {
	t.Parallel()
	w := hookWorld(t, hookPolicy+hookCeiling)
	res, err := w.eng.MapPrincipal(context.Background(), "system:webhook:icinga-alert", "", "", hookTypes)
	if err != nil {
		t.Fatalf("MapPrincipal: %v", err)
	}
	if typeOf(res, "project") != nil || typeOf(res, "ticket") != nil {
		t.Errorf("uncapped access reported for the hook name: %+v", res.Types)
	}
}

// TestMapPrincipalAs_CeilingRemovesEveryoneGrant covers the same defect on
// --as: a baseline whose read allowlist omits a type the everyone role reads.
func TestMapPrincipalAs_CeilingRemovesEveryoneGrant(t *testing.T) {
	t.Parallel()
	w := buildWorld(t, groundingPolicy+`
client_baselines:
  apps:
    applies_to: [app]
    read: [ticket]
`, []ent{{"PERS-ALICE", "person"}, {"PROJ", "project"}, {"TKT-1", "ticket"}}, nil)

	direct := mapAs(t, w, aclmap.ClientView{})
	if typeOf(direct, "project") == nil {
		t.Fatal("precondition: everyone reads project without a client view")
	}
	viaApp := mapAs(t, w, aclmap.ClientView{PrincipalType: "app"})
	if ta := typeOf(viaApp, "project"); ta != nil {
		t.Errorf("everyone read on project reported through a ceiling that removes it: %+v", ta)
	}
}

func TestMapWebhook_CheckMapping(t *testing.T) {
	t.Parallel()
	alert := &aclmap.WebhookMapping{
		FindType: "incident", Match: []string{"host"}, CreateType: "incident", Updates: true,
	}
	for _, tc := range []struct {
		name     string
		policy   string
		mapping  *aclmap.WebhookMapping
		wantHeld map[string]bool
		// wantHidden are the match fields the ceiling hides (BUG-KJ5D18).
		wantHidden []string
	}{
		{
			name: "capped hook holds what it needs", policy: hookPolicy + hookCeiling, mapping: alert,
			wantHeld: map[string]bool{"read incident": true, "create incident": true, "update incident": true},
		},
		{
			name: "find on a type the ceiling hides", policy: hookPolicy + hookCeiling,
			mapping:  &aclmap.WebhookMapping{FindType: "ticket", Updates: true},
			wantHeld: map[string]bool{"read ticket": false, "update ticket": false},
		},
		{
			name: "match field the ceiling redacts", mapping: alert,
			policy:     hookPolicy + hookCeiling + "    redact:\n      incident: [host]\n",
			wantHeld:   map[string]bool{"read incident": true, "create incident": true, "update incident": true},
			wantHidden: []string{"host"},
		},
		{
			name: "match field outside the ceiling allowlist", mapping: alert,
			policy:     hookPolicy + hookCeiling + "    visible:\n      incident: [title]\n",
			wantHeld:   map[string]bool{"read incident": true, "create incident": true, "update incident": true},
			wantHidden: []string{"host"},
		},
		{
			name: "match field the ceiling leaves visible", mapping: alert,
			policy:   hookPolicy + hookCeiling + "    redact:\n      incident: [title]\n",
			wantHeld: map[string]bool{"read incident": true, "create incident": true, "update incident": true},
		},
		{
			// The unheld read already explains the failure.
			name:     "hidden match field on an unreadable find type",
			policy:   hookPolicy + hookCeiling + "    redact:\n      ticket: [host]\n",
			mapping:  &aclmap.WebhookMapping{FindType: "ticket", Match: []string{"host"}},
			wantHeld: map[string]bool{"read ticket": false},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			res := mapHook(t, hookWorld(t, tc.policy), tc.mapping)
			c := res.WebhookCheck
			if c == nil {
				t.Fatal("WebhookCheck missing")
			}
			if !slices.Equal(c.HiddenMatchFields, tc.wantHidden) {
				t.Errorf("HiddenMatchFields = %v, want %v", c.HiddenMatchFields, tc.wantHidden)
			}
			got := map[string]bool{}
			for _, n := range c.Needs {
				got[n.Verb+" "+n.Type] = n.Held
			}
			if len(got) != len(tc.wantHeld) {
				t.Errorf("needs = %v, want %v", got, tc.wantHeld)
			}
			for k, want := range tc.wantHeld {
				if held, ok := got[k]; !ok || held != want {
					t.Errorf("need %q held = %v (present %v), want %v", k, held, ok, want)
				}
			}
			wantOK := len(tc.wantHidden) == 0
			for _, h := range tc.wantHeld {
				wantOK = wantOK && h
			}
			if c.OK() != wantOK {
				t.Errorf("OK() = %v, want %v", c.OK(), wantOK)
			}
		})
	}
}

// TestMapPrincipal_ReportsFaceOnlyGrant pins that a `type@face` grant appears
// in the map. The baseline is decided per type, so without Faces the grant
// was invisible and an expectations file could not catch it.
func TestMapPrincipal_ReportsFaceOnlyGrant(t *testing.T) {
	t.Parallel()
	w := withFaces(t, buildWorld(t, `
roles:
  drafter:
    read: [terugkerend]
    update: [terugkerend@draft]
assignments:
  PERS-D: drafter
`, []ent{{"PERS-D", "person"}}, nil), facedTerugkerend)

	res, err := w.eng.MapPrincipal(context.Background(), "PERS-D", "", "", []string{"terugkerend"})
	if err != nil {
		t.Fatalf("MapPrincipal: %v", err)
	}
	ta := typeOf(res, "terugkerend")
	if ta == nil {
		t.Fatal("terugkerend missing from the map")
	}
	if got := ta.Faces["draft"]; !slices.Equal(got, []string{"update"}) {
		t.Errorf("Faces[draft] = %v, want [update]", got)
	}
	if _, ok := ta.Faces["published"]; ok {
		t.Errorf("Faces[published] = %v, want absent", ta.Faces["published"])
	}
}

// TestMapPrincipal_FacedTypeDecidedPerFace pins that a faced type is reported
// as the runtime decides it: a face-scoped read is not a read of the whole
// type, and a bare or "*" write grant covers no face of a faced type.
func TestMapPrincipal_FacedTypeDecidedPerFace(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		grants   string
		wantWide []string
		wantFace map[string][]string
	}{
		{
			name:     "face-scoped read",
			grants:   "read: [terugkerend@draft]",
			wantFace: map[string][]string{"draft": {"read"}},
		},
		{
			name:     "wildcard write covers no face",
			grants:   "read: [\"*\"]\n    update: [\"*\"]",
			wantWide: []string{"read"},
		},
		{
			name:     "bare write covers no face",
			grants:   "read: [terugkerend]\n    update: [terugkerend]",
			wantWide: []string{"read"},
		},
		{
			name:     "face grants on every face",
			grants:   "read: [terugkerend]\n    update: [terugkerend@draft, terugkerend@published]",
			wantWide: []string{"read"},
			wantFace: map[string][]string{"draft": {"update"}, "published": {"update"}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			w := withFaces(t, buildWorld(t, `
roles:
  r:
    `+tc.grants+`
assignments:
  PERS-D: r
`, []ent{{"PERS-D", "person"}}, nil), facedTerugkerend)
			res, err := w.eng.MapPrincipal(context.Background(), "PERS-D", "", "", []string{"terugkerend"})
			if err != nil {
				t.Fatalf("MapPrincipal: %v", err)
			}
			if got := typeVerbs(res, "terugkerend"); !slices.Equal(got, tc.wantWide) {
				t.Errorf("type-wide verbs = %v, want %v", got, tc.wantWide)
			}
			var faces map[string][]string
			if ta := typeOf(res, "terugkerend"); ta != nil {
				faces = ta.Faces
			}
			if len(faces) != len(tc.wantFace) {
				t.Fatalf("Faces = %v, want %v", faces, tc.wantFace)
			}
			for face, want := range tc.wantFace {
				if got := faces[face]; !slices.Equal(got, want) {
					t.Errorf("Faces[%s] = %v, want %v", face, got, want)
				}
			}
		})
	}
}

// TestMapWebhook_WildcardWriteOnFacedFindIsNotHeld pins the mapping check for
// a faced find type: "*" covers no face on a write, so the runtime refuses
// the update and the check must say so.
func TestMapWebhook_WildcardWriteOnFacedFindIsNotHeld(t *testing.T) {
	t.Parallel()
	w := withFaces(t, buildWorld(t, `
roles:
  hook:
    read: ["*"]
    create: ["*"]
    update: ["*"]
assignments:
  system:webhook:icinga-alert: hook
`, nil, nil), facedTerugkerend)
	res, err := w.eng.MapWebhook(context.Background(), "icinga-alert", &aclmap.WebhookMapping{
		FindType: "terugkerend", FindFace: "draft", Match: []string{"title"},
		CreateType: "terugkerend", Updates: true,
	}, "", "", []string{"terugkerend"}, aclmap.Extras{})
	if err != nil {
		t.Fatalf("MapWebhook: %v", err)
	}
	for _, n := range res.WebhookCheck.Needs {
		want := n.Verb == "read"
		if n.Held != want {
			t.Errorf("%s %s@%s held = %v, want %v", n.Verb, n.Type, n.Face, n.Held, want)
		}
	}
	if res.WebhookCheck.OK() {
		t.Error("check OK, want a failure: the hook cannot write the draft face")
	}
}

func TestMapWebhook_RejectsEmptyID(t *testing.T) {
	t.Parallel()
	w := hookWorld(t, hookPolicy)
	if _, err := w.eng.MapWebhook(context.Background(), "  ", nil, "", "", hookTypes, aclmap.Extras{}); err == nil {
		t.Fatal("MapWebhook with an empty id succeeded, want an error")
	}
}
