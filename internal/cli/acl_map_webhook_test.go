package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/output"
)

// `rela acl map --webhook` and `--expect` (TKT-J3RDJ4).

func hookMeta() *metamodel.Metamodel {
	sp := map[string]metamodel.PropertyDef{}
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"incident": {Label: "Incident", IDPrefix: "INC-", Properties: map[string]metamodel.PropertyDef{
				"host": {Type: "string"},
			}},
			"ticket":  {Label: "Ticket", IDPrefix: "TKT-", Properties: sp},
			"project": {Label: "Project", IDPrefix: "PROJ-", Properties: sp},
		},
		Relations: map[string]metamodel.RelationDef{
			"affects": {From: []string{"incident", "ticket"}, To: []string{"project"}},
		},
	}
}

const hookRolesPolicy = `
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

const hookBaselinePolicy = hookRolesPolicy + `
client_baselines:
  hooks:
    applies_to: [webhook]
    read: [incident]
    create: [incident]
    update: [incident]
    deny_delete: ["*"]
    deny_permissions: ["*"]
`

const hookDataEntry = `
webhooks:
  icinga-alert:
    find:
      type: incident
      match: [host]
    create_if_missing:
      properties:
        host: "{{host}}"
    then:
      - set:
          host: "{{host}}"
`

// cappedExpectation is the access hookBaselinePolicy leaves the hook.
const cappedExpectation = `
types:
  incident: [read, create, update]
permissions: []
relations:
  affects:
    create: [incident]
    update: [incident]
`

func hookServices(t *testing.T, policy string) *readServices {
	t.Helper()
	svc := aclTestServices(t, hookMeta(), policy)
	svc.Config = loaderWith(t, hookDataEntry)
	return svc
}

func writeExpectFile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "expect.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write expectations: %v", err)
	}
	return path
}

func TestACLMapWebhook_Text(t *testing.T) {
	svc := hookServices(t, hookBaselinePolicy)
	buf := withOutput(t, output.FormatTable)

	if err := (&ACLMapCmd{Webhook: "icinga-alert"}).Run(context.Background(), svc); err != nil {
		t.Fatalf("map --webhook: %v", err)
	}
	got := buf.String()
	for _, want := range []string{
		"webhook icinga-alert (system:webhook:icinga-alert)",
		`as client type "webhook" — client baseline "hooks" is applied below.`,
		"create (all incident)",
		"permissions: none",
		"affects: create from incident; update from incident",
		"read incident: held",
		"create incident: held",
		"update incident: held",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, got)
		}
	}
	for _, unwanted := range []string{"project", "ticket", "delete ("} {
		if strings.Contains(got, unwanted) {
			t.Errorf("capped output mentions %q\nfull output:\n%s", unwanted, got)
		}
	}
}

// Without a webhook baseline the header says the hook is not capped, rather
// than implying a ceiling that does not exist.
func TestACLMapWebhook_TextUncapped(t *testing.T) {
	svc := hookServices(t, hookRolesPolicy)
	buf := withOutput(t, output.FormatTable)

	if err := (&ACLMapCmd{Webhook: "icinga-alert"}).Run(context.Background(), svc); err != nil {
		t.Fatalf("map --webhook: %v", err)
	}
	if got, want := buf.String(), "no client baseline matches, so access is not capped"; !strings.Contains(got, want) {
		t.Errorf("output missing %q\nfull output:\n%s", want, got)
	}
}

func TestACLMap_FlagErrors(t *testing.T) {
	expect := writeExpectFile(t, cappedExpectation)
	for _, tc := range []struct {
		name string
		cmd  ACLMapCmd
		want string
	}{
		{"webhook with principal", ACLMapCmd{Webhook: "icinga-alert", Principal: "x"}, "--webhook fixes"},
		{"webhook with as", ACLMapCmd{Webhook: "icinga-alert", As: "app"}, "--webhook fixes"},
		{"hook principal with as", ACLMapCmd{Principal: "system:webhook:icinga-alert", As: "app"}, "always has principal_type webhook"},
		{"expect without a principal", ACLMapCmd{Expect: expect}, "requires --principal or --webhook"},
		{"expect with verb", ACLMapCmd{Webhook: "icinga-alert", Expect: expect, Verb: "read"}, "--verb or --type"},
		{"expect with type", ACLMapCmd{Webhook: "icinga-alert", Expect: expect, Type: "incident"}, "--verb or --type"},
		{"unknown webhook", ACLMapCmd{Webhook: "nope"}, `unknown webhook "nope"; data-entry.yaml declares: icinga-alert`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := hookServices(t, hookBaselinePolicy)
			withOutput(t, output.FormatTable)
			err := tc.cmd.Run(context.Background(), svc)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want one containing %q", err, tc.want)
			}
		})
	}
}

func TestACLMapExpect_MatchExitsZero(t *testing.T) {
	svc := hookServices(t, hookBaselinePolicy)
	buf := withOutput(t, output.FormatTable)

	cmd := &ACLMapCmd{Webhook: "icinga-alert", Expect: writeExpectFile(t, cappedExpectation)}
	if err := cmd.Run(context.Background(), svc); err != nil {
		t.Fatalf("matching expectations returned %v, want nil", err)
	}
	if got := buf.String(); !strings.Contains(got, "access matches") {
		t.Errorf("output missing the match line:\n%s", got)
	}
}

// TestACLMapExpect_DiffExitsOne pins the CI gate: without the baseline the
// hook holds more than the file lists, and the diff names each surplus with
// the grant that confers it.
func TestACLMapExpect_DiffExitsOne(t *testing.T) {
	svc := hookServices(t, hookRolesPolicy)
	buf := withOutput(t, output.FormatTable)

	cmd := &ACLMapCmd{Webhook: "icinga-alert", Expect: writeExpectFile(t, cappedExpectation+`
  # listed but never granted
`)}
	err := cmd.Run(context.Background(), svc)
	if code := exitCode(err); code != 1 {
		t.Fatalf("Run = %v (exit %d), want exit 1", err, code)
	}
	got := buf.String()
	for _, want := range []string{
		"+ types.incident: delete (role icinga-webhook [global])",
		"+ types.project: read (role everyone [global])",
		"+ types.ticket: read",
		"+ permissions: comment.add",
		"+ relations.affects: update from ticket",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("diff missing %q\nfull output:\n%s", want, got)
		}
	}
}

func TestACLMapExpect_MissingGrant(t *testing.T) {
	svc := hookServices(t, hookBaselinePolicy)
	buf := withOutput(t, output.FormatTable)

	cmd := &ACLMapCmd{Webhook: "icinga-alert", Expect: writeExpectFile(t, `
types:
  incident: [read, create, update, delete]
relations:
  affects:
    create: [incident]
    update: [incident]
`)}
	if code := exitCode(cmd.Run(context.Background(), svc)); code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if got := buf.String(); !strings.Contains(got, "- types.incident: delete") {
		t.Errorf("diff missing the expected-but-not-held line:\n%s", got)
	}
}

// TestACLMapExpect_EmptyFileExpectsNothing pins that an empty or
// comment-only file asserts no access, rather than failing to parse.
func TestACLMapExpect_EmptyFileExpectsNothing(t *testing.T) {
	svc := hookServices(t, hookBaselinePolicy)
	buf := withOutput(t, output.FormatTable)

	cmd := &ACLMapCmd{Webhook: "icinga-alert", Expect: writeExpectFile(t, "# no access at all\n")}
	if code := exitCode(cmd.Run(context.Background(), svc)); code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if got := buf.String(); !strings.Contains(got, "+ types.incident: create") {
		t.Errorf("diff missing the held access:\n%s", got)
	}
}

func TestACLMapExpect_JSON(t *testing.T) {
	svc := hookServices(t, hookRolesPolicy)
	buf := withOutput(t, output.FormatJSON)

	cmd := &ACLMapCmd{Webhook: "icinga-alert", Expect: writeExpectFile(t, cappedExpectation)}
	if code := exitCode(cmd.Run(context.Background(), svc)); code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	var got struct {
		Webhook     string   `json:"webhook"`
		As          string   `json:"as"`
		Permissions []string `json:"permissions"`
		Expectation struct {
			Match      bool `json:"match"`
			Unexpected []struct {
				Section, Key, Value string
			} `json:"unexpected"`
		} `json:"expectation"`
	}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, buf.String())
	}
	if got.Webhook != "icinga-alert" || got.As != "webhook" || got.Expectation.Match {
		t.Errorf("json = %+v, want webhook icinga-alert, as webhook, match false", got)
	}
	if len(got.Expectation.Unexpected) == 0 {
		t.Error("unexpected list is empty")
	}
}

func TestACLMapExpect_BadFile(t *testing.T) {
	for _, tc := range []struct {
		name, body, want string
	}{
		{"unknown top-level key", "type: {}\n", "field type not found"},
		{"unknown entity type", "types: {nope: [read]}\n", `unknown entity type "nope"`},
		{"bad verb", "types: {incident: [write]}\n", `verb "write"`},
		{"rename", "types: {incident: [rename]}\n", "rename is not reported"},
		{"undeclared face", "types: {incident@draft: [update]}\n", `declares no face "draft"`},
		{"unknown relation", "relations: {nope: {create: [incident]}}\n", `unknown relation type "nope"`},
		{"relation verb", "relations: {affects: {read: [incident]}}\n", `verb "read"`},
		{"relation source", "relations: {affects: {create: [project]}}\n", `"project" is not a source type`},
		{"wildcard permission", "permissions: ['*']\n", "not a permission name"},
		{"two documents", "types: {}\n---\npermissions: []\n", "one YAML document"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := hookServices(t, hookBaselinePolicy)
			withOutput(t, output.FormatTable)
			cmd := &ACLMapCmd{Webhook: "icinga-alert", Expect: writeExpectFile(t, tc.body)}
			err := cmd.Run(context.Background(), svc)
			if err == nil || exitCode(err) == 1 || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want a load error containing %q", err, tc.want)
			}
		})
	}
}

func TestACLMapExpect_NoPolicyIsAnError(t *testing.T) {
	svc := hookServices(t, "")
	withOutput(t, output.FormatTable)
	cmd := &ACLMapCmd{Webhook: "icinga-alert", Expect: writeExpectFile(t, cappedExpectation)}
	if err := cmd.Run(context.Background(), svc); err == nil || !strings.Contains(err.Error(), "needs acl.yaml") {
		t.Fatalf("err = %v, want the no-policy error", err)
	}
}

// hookUnreadablePolicy caps the hook below its mapping: the find type is not
// readable, so the find never matches and every delivery creates again.
const hookUnreadablePolicy = hookRolesPolicy + `
client_baselines:
  hooks:
    applies_to: [webhook]
    read: [ticket]
    create: [incident]
`

// TestACLMapExpect_FailingWebhookCheckExitsOne pins that a ceiling which
// breaks the hook fails the gate, and says so after the comparison.
func TestACLMapExpect_FailingWebhookCheckExitsOne(t *testing.T) {
	svc := hookServices(t, hookUnreadablePolicy)
	buf := withOutput(t, output.FormatTable)

	cmd := &ACLMapCmd{Webhook: "icinga-alert", Expect: writeExpectFile(t, cappedExpectation)}
	if code := exitCode(cmd.Run(context.Background(), svc)); code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	got := buf.String()
	for _, want := range []string{"read incident: NOT HELD", "mapping check failed"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q:\n%s", want, got)
		}
	}
}

// Without --expect a failed mapping check still exits 1, so `acl map
// --webhook` alone is a usable CI check.
func TestACLMapWebhook_FailingCheckExitsOneWithoutExpect(t *testing.T) {
	svc := hookServices(t, hookUnreadablePolicy)
	withOutput(t, output.FormatTable)

	if code := exitCode((&ACLMapCmd{Webhook: "icinga-alert"}).Run(context.Background(), svc)); code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
}

func TestACLMapExpect_Principal(t *testing.T) {
	svc := hookServices(t, hookBaselinePolicy)
	withOutput(t, output.FormatTable)

	// The hook's name alone gets the same ceiling as --webhook.
	cmd := &ACLMapCmd{Principal: "system:webhook:icinga-alert", Expect: writeExpectFile(t, cappedExpectation)}
	if err := cmd.Run(context.Background(), svc); err != nil {
		t.Fatalf("Run = %v, want nil", err)
	}
}

func TestACLAudit_WebhookUncapped(t *testing.T) {
	for _, tc := range []struct {
		name   string
		policy string
		want   bool
	}{
		{"no webhook baseline", hookRolesPolicy, true},
		{"webhook baseline", hookBaselinePolicy, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := hookServices(t, tc.policy)
			buf := withOutput(t, output.FormatTable)
			if err := (&ACLAuditCmd{NoClassification: true}).Run(svc); err != nil {
				t.Fatalf("audit: %v", err)
			}
			if got := strings.Contains(buf.String(), "A15-webhook-uncapped"); got != tc.want {
				t.Errorf("A15 reported = %v, want %v\nfull output:\n%s", got, tc.want, buf.String())
			}
		})
	}
}

// An unparsable data-entry.yaml cannot pass the A15 gate: with --fail-on the
// audit fails, without it the audit warns.
func TestACLAudit_UnreadableDataEntry(t *testing.T) {
	for _, tc := range []struct {
		name    string
		failOn  string
		wantErr bool
	}{
		{"gated", "medium", true},
		{"advisory", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := aclTestServices(t, hookMeta(), hookRolesPolicy)
			svc.Config = loaderWith(t, "webhooks: [not, a, map\n")
			buf := withOutput(t, output.FormatTable)
			err := (&ACLAuditCmd{NoClassification: true, FailOn: tc.failOn}).Run(svc)
			if (err != nil) != tc.wantErr {
				t.Fatalf("audit error = %v, want error: %v", err, tc.wantErr)
			}
			if !tc.wantErr && !strings.Contains(buf.String(), "skipping the webhook ceiling check") {
				t.Errorf("advisory audit did not warn:\n%s", buf.String())
			}
		})
	}
}
