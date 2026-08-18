package appbuild_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// redactionMetamodel declares a type with a property a policy can hide.
const redactionMetamodel = `version: "1.0"
entities:
  person:
    label: Person
    plural: people
    id_prefix: "PERS-"
    id_type: sequential
    properties:
      name:
        type: string
      salary:
        type: string
relations: {}
`

// redactionPolicy hides person.salary from the `viewer` role while leaving the
// row itself readable. That split is the point: row gating alone would let
// every property through, so a test asserting only "the entity is visible"
// would pass with no redaction at all.
const redactionPolicy = `roles:
  viewer:
    read: ["*"]
    visible:
      person:
        - field: name
assignments:
  bob: viewer
`

// writeRedactionProject lays down a project whose policy redacts a field, plus
// one entity carrying a value that must not escape.
func writeRedactionProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeMetamodelBody(t, root, redactionMetamodel)
	writePolicy(t, root, redactionPolicy)

	dir := filepath.Join(root, "entities", "people")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nid: PERS-1\ntype: person\nname: Alice\nsalary: \"99000\"\n---\n"
	if err := os.WriteFile(filepath.Join(dir, "PERS-1.md"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}

// bobCtx returns a ctx carrying the principal the policy assigns to `viewer`,
// stamped with the tool the surface under test actually runs as. Both fields
// are required: acl rejects a half-stamped principal and the row gate then
// fails closed, which would make a redaction test pass for the wrong reason
// (nothing returned at all).
func bobCtx(tool string) context.Context {
	return principal.With(context.Background(), principal.Principal{User: "bob", Tool: tool})
}

// TestGatedReads_RedactsHiddenField closes the field-level half of the
// GatedReads limitation (TKT-0XL8MF).
//
// GatedReads previously passed a nil redactor, so this path was ROW gating
// only: an MCP caller who could read `person` at all received every property,
// including ones the same role sees redacted in the UI. Against that code this
// test fails on the salary assertion.
func TestGatedReads_RedactsHiddenField(t *testing.T) {
	root := writeRedactionProject(t)
	svc, err := appbuildOnDisk(t, root)
	if err != nil {
		t.Fatalf("appbuild.New: %v", err)
	}
	defer svc.Close()

	got, err := svc.GatedReads().Reader.GetEntity(bobCtx(principal.ToolMCP), "PERS-1")
	if err != nil {
		t.Fatalf("GetEntity: %v", err)
	}
	if got == nil {
		t.Fatal("entity is nil — the row gate hid a row the policy permits reading")
	}
	// The visible property must survive: over-redaction is as wrong as none.
	if got.GetString("name") != "Alice" {
		t.Errorf("name = %q, want \"Alice\" — a permitted field was redacted", got.GetString("name"))
	}
	if v := got.GetString("salary"); v != "" {
		t.Errorf("salary = %q, want redacted to empty — a `visible:`-hidden value "+
			"reached an MCP read; row gating alone does not redact fields", v)
	}
}

// TestScheduledLuaWriteDeps_RedactsHiddenField closes RR-7408F5, the same gap
// on the scheduler path. A scheduled job's reads flow into whatever the job
// sends onward (a prompt, a webhook), so an unredacted property here leaves
// the system entirely.
func TestScheduledLuaWriteDeps_RedactsHiddenField(t *testing.T) {
	root := writeRedactionProject(t)
	svc, err := appbuildOnDisk(t, root)
	if err != nil {
		t.Fatalf("appbuild.New: %v", err)
	}
	defer svc.Close()

	deps := svc.ScheduledLuaWriteDeps()
	if deps.VisibleReader == nil {
		t.Fatal("ScheduledLuaWriteDeps has no VisibleReader")
	}
	got, err := deps.VisibleReader.GetEntity(bobCtx(principal.ToolScheduler), "PERS-1")
	if err != nil {
		t.Fatalf("GetEntity: %v", err)
	}
	if got == nil {
		t.Fatal("entity is nil — the row gate hid a row the policy permits reading")
	}
	if got.GetString("name") != "Alice" {
		t.Errorf("name = %q, want \"Alice\" — a permitted field was redacted", got.GetString("name"))
	}
	if v := got.GetString("salary"); v != "" {
		t.Errorf("salary = %q, want redacted to empty — a scheduled job read a "+
			"`visible:`-hidden value (RR-7408F5)", v)
	}
}

// TestNoPolicy_RedactorHidesNothing pins the byte-parity path. A project with
// no acl.yaml must behave exactly as it did pre-ACL; if buildFieldRedactor
// returned a policy-backed redactor (or nil) here instead of NopRedactor, this
// would regress silently for every existing project.
func TestNoPolicy_RedactorHidesNothing(t *testing.T) {
	root := t.TempDir()
	writeMetamodelBody(t, root, redactionMetamodel)
	// deliberately no acl.yaml

	dir := filepath.Join(root, "entities", "people")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nid: PERS-1\ntype: person\nname: Alice\nsalary: \"99000\"\n---\n"
	if err := os.WriteFile(filepath.Join(dir, "PERS-1.md"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	svc, err := appbuildOnDisk(t, root)
	if err != nil {
		t.Fatalf("appbuild.New: %v", err)
	}
	defer svc.Close()

	got, err := svc.GatedReads().Reader.GetEntity(bobCtx(principal.ToolMCP), "PERS-1")
	if err != nil {
		t.Fatalf("GetEntity: %v", err)
	}
	if got == nil {
		t.Fatal("entity is nil — no policy is configured, nothing may be hidden")
	}
	if got.GetString("salary") != "99000" {
		t.Errorf("salary = %q, want \"99000\" — with no acl.yaml the read must be "+
			"byte-identical to pre-ACL behavior", got.GetString("salary"))
	}
}

// TestPolicyWithoutAffordanceGrants_HidesNothing covers the third
// buildFieldRedactor branch: an acl.yaml that gates ROWS but declares no
// `visible:` grants. HasAffordanceGrants is false there, so the redactor must
// be NopRedactor — not a policy-backed one that hides nothing by accident, and
// not an error. Row-only policies are a normal, common configuration.
func TestPolicyWithoutAffordanceGrants_HidesNothing(t *testing.T) {
	root := t.TempDir()
	writeMetamodelBody(t, root, redactionMetamodel)
	writePolicy(t, root, `roles:
  viewer:
    read: ["*"]
assignments:
  bob: viewer
`)

	dir := filepath.Join(root, "entities", "people")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nid: PERS-1\ntype: person\nname: Alice\nsalary: \"99000\"\n---\n"
	if err := os.WriteFile(filepath.Join(dir, "PERS-1.md"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	svc, err := appbuildOnDisk(t, root)
	if err != nil {
		t.Fatalf("appbuild.New: %v", err)
	}
	defer svc.Close()

	got, err := svc.GatedReads().Reader.GetEntity(bobCtx(principal.ToolMCP), "PERS-1")
	if err != nil {
		t.Fatalf("GetEntity: %v", err)
	}
	if got == nil {
		t.Fatal("entity is nil — the policy grants read on this row")
	}
	if got.GetString("salary") != "99000" {
		t.Errorf("salary = %q, want \"99000\" — a policy with no `visible:` grants "+
			"must not redact anything", got.GetString("salary"))
	}
}
