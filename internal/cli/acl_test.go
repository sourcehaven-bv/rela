package cli

import (
	"encoding/json"
	stderrors "errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/errors"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

func stringContainsACL(s, sub string) bool { return strings.Contains(s, sub) }

// aclTestServices wires a cliServices whose project Root is a real temp dir
// (acl.LoadPolicy reads acl.yaml from the OS filesystem). aclYAML, when
// non-empty, is written to <root>/acl.yaml.
func aclTestServices(t *testing.T, meta *metamodel.Metamodel, aclYAML string) *cliServices {
	t.Helper()
	root := t.TempDir()
	if aclYAML != "" {
		if err := os.WriteFile(filepath.Join(root, "acl.yaml"), []byte(aclYAML), 0o600); err != nil {
			t.Fatalf("write acl.yaml: %v", err)
		}
	}
	paths := &project.Context{Root: root, CacheDir: filepath.Join(root, ".rela")}
	svc, err := newCLIServicesFromAppbuild(
		appbuildtest.New(meta, appbuildtest.WithFS(storage.NewOsFS(), paths)),
	)
	if err != nil {
		t.Fatalf("build cli services: %v", err)
	}
	return svc
}

func aclTestMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Label: "Ticket", IDPrefix: "TKT-", Properties: map[string]metamodel.PropertyDef{}},
		},
	}
}

// A policy with an un-gated member-of + privileged assignment produces an A1
// high finding; --exit-code then returns a non-zero ExitError.
func TestACLAudit_ExitCodeOnHigh(t *testing.T) {
	const policy = `
roles:
  editor:
    create: [ticket]
    read: [ticket]
assignments:
  engineering: editor
`
	svc := aclTestServices(t, aclTestMeta(), policy)
	withOutput(t, output.FormatTable)

	cmd := &ACLAuditCmd{ExitCode: true}
	err := cmd.Run(svc)
	var exitErr *errors.ExitError
	if !stderrors.As(err, &exitErr) {
		t.Fatalf("expected ExitError for a high finding, got %v", err)
	}
	if exitErr.Code == 0 {
		t.Errorf("ExitError code = 0, want non-zero")
	}
}

// --fail-on sets the exit-code threshold. A policy whose worst finding is
// medium (A9 wildcard-write) exits 0 at the default high gate but non-zero at
// --fail-on=medium / =any — so medium/low warnings don't break CI unless the
// operator opts in.
func TestACLAudit_FailOnThreshold(t *testing.T) {
	// A9 wildcard-write (medium) is the worst finding; no critical/high.
	const policy = `
roles:
  power:
    create: ["*"]
    update: ["*"]
    delete: ["*"]
    read: ["*"]
`
	cases := []struct {
		name      string
		cmd       ACLAuditCmd
		wantError bool
	}{
		{"advisory default", ACLAuditCmd{}, false},
		{"exit-code alias (=high)", ACLAuditCmd{ExitCode: true}, false},
		{"fail-on high", ACLAuditCmd{FailOn: "high"}, false},
		{"fail-on medium", ACLAuditCmd{FailOn: "medium"}, true},
		{"fail-on any", ACLAuditCmd{FailOn: "any"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := aclTestServices(t, aclTestMeta(), policy)
			withOutput(t, output.FormatTable)
			err := tc.cmd.Run(svc)
			var exitErr *errors.ExitError
			gotError := stderrors.As(err, &exitErr)
			if gotError != tc.wantError {
				t.Errorf("%s: gotError=%v (err=%v), want %v", tc.name, gotError, err, tc.wantError)
			}
		})
	}
}

// An invalid --fail-on value is rejected with a clear error (belt-and-suspenders
// behind Kong's enum tag).
func TestACLAudit_FailOnInvalid(t *testing.T) {
	svc := aclTestServices(t, aclTestMeta(), "roles:\n  everyone:\n    read: [\"*\"]\n")
	withOutput(t, output.FormatTable)
	cmd := &ACLAuditCmd{FailOn: "bogus"}
	if err := cmd.Run(svc); err == nil {
		t.Fatal("expected an error for --fail-on=bogus, got nil")
	}
}

// A clean, well-gated policy produces no findings; --exit-code returns nil.
func TestACLAudit_CleanPolicyExitsZero(t *testing.T) {
	const policy = `
user_entity_type: person
roles:
  admin:
    create: [ticket]
    update: [ticket]
    delete: [ticket]
    read: [ticket]
    permissions: [delegate-membership]
  everyone:
    read: ["*"]
assignments:
  ops-team: admin
role_relations:
  member-of:
    requires_permission: delegate-membership
`
	meta := aclTestMeta()
	meta.Entities["person"] = metamodel.EntityDef{Label: "Person", IDPrefix: "PERS-", Properties: map[string]metamodel.PropertyDef{}}
	meta.Relations = map[string]metamodel.RelationDef{"member-of": {From: []string{"person"}, To: []string{"group"}}}
	meta.Entities["group"] = metamodel.EntityDef{Label: "Group", IDPrefix: "GRP-", Properties: map[string]metamodel.PropertyDef{}}

	svc := aclTestServices(t, meta, policy)
	buf := withOutput(t, output.FormatTable)

	cmd := &ACLAuditCmd{ExitCode: true}
	if err := cmd.Run(svc); err != nil {
		t.Fatalf("clean policy must exit 0, got %v", err)
	}
	if got := buf.String(); !stringContainsACL(got, "no findings") {
		t.Errorf("expected 'no findings' in output, got: %s", got)
	}
}

// JSON output uses the AnalysisResult envelope with the findings as details.
func TestACLAudit_JSONOutput(t *testing.T) {
	// everyone with a write grant (covered by read, so Validate accepts it) →
	// A3 critical from the audit.
	const policy = `
roles:
  everyone:
    update: [ticket]
    read: [ticket]
`
	svc := aclTestServices(t, aclTestMeta(), policy)
	buf := withOutput(t, output.FormatJSON)

	cmd := &ACLAuditCmd{}
	if err := cmd.Run(svc); err != nil { // no --exit-code: must not error
		t.Fatalf("audit run error: %v", err)
	}
	var result output.AnalysisResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal JSON: %v", err)
	}
	if result.Status != "warning" {
		t.Errorf("status = %q, want warning", result.Status)
	}
	if result.Count == 0 {
		t.Errorf("expected non-zero finding count for everyone:update")
	}
}

// No acl.yaml → success message, no error (nothing to audit).
func TestACLAudit_NoPolicyFile(t *testing.T) {
	svc := aclTestServices(t, aclTestMeta(), "") // no acl.yaml written
	buf := withOutput(t, output.FormatTable)

	cmd := &ACLAuditCmd{ExitCode: true}
	if err := cmd.Run(svc); err != nil {
		t.Fatalf("missing acl.yaml must not error, got %v", err)
	}
	if got := buf.String(); !stringContainsACL(got, "nothing to audit") {
		t.Errorf("expected 'nothing to audit', got: %s", got)
	}
}
