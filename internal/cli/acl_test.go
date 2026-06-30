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
