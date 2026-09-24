//go:build !postgres && !memorybackend && !sqlite

// fsstore-only for the reason fieldredaction_test.go gives: the fixture is
// markdown written to disk.

package appbuild_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

const traversalMetamodel = `version: "1.0"
entities:
  ticket:
    label: Ticket
    plural: tickets
    id_prefix: "T-"
    id_type: sequential
    properties:
      status:
        type: string
  person:
    label: Person
    plural: people
    id_prefix: "P-"
    id_type: sequential
    properties:
      name:
        type: string
relations:
  owned-by:
    from: [ticket]
    to: [person]
validations:
  - name: done-needs-owner
    entity_type: ticket
    when_condition: "entity.status == 'done'"
    then_condition: "related(entity, 'owned-by')"
    severity: error
`

// bob reads tickets but not people, so to him an owner does not exist.
const traversalPolicy = `roles:
  viewer:
    read: [ticket]
assignments:
  bob: viewer
`

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// The MCP validator answers related() through the caller's gate: a hidden
// target is a nonexistent one, the same as on a read.
func TestGatedReads_ValidatorTraversalUsesCallerGate(t *testing.T) {
	root := t.TempDir()
	writeMetamodelBody(t, root, traversalMetamodel)
	writePolicy(t, root, traversalPolicy)
	for _, id := range []string{"T-1", "T-2"} {
		writeFile(t, filepath.Join(root, "entities", "tickets", id+".md"),
			"---\nid: "+id+"\ntype: ticket\nstatus: done\n---\n")
	}
	writeFile(t, filepath.Join(root, "entities", "people", "P-1.md"), "---\nid: P-1\ntype: person\nname: p\n---\n")
	writeFile(t, filepath.Join(root, "relations", "T-1--owned-by--P-1.md"),
		"---\nfrom: T-1\nrelation: owned-by\nto: P-1\n---\n")

	svc, err := appbuildOnDisk(t, root)
	if err != nil {
		t.Fatalf("appbuild.New: %v", err)
	}
	defer svc.Close()
	rule := metamodel.ValidationRule{
		Name: "done-needs-owner", EntityType: "ticket",
		WhenCondition: "entity.status == 'done'", ThenCondition: "related(entity, 'owned-by')",
		Severity: "error",
	}

	raw, err := svc.Validator().CheckRule(bobCtx(principal.ToolCLI), rule)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(raw, []string{"T-2"}) {
		t.Fatalf("system validator violations = %v, want [T-2]", raw)
	}

	gated, err := svc.GatedReads().Validator.CheckRule(bobCtx(principal.ToolMCP), rule)
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(gated)
	if !slices.Equal(gated, []string{"T-1", "T-2"}) {
		t.Fatalf("gated violations = %v, want [T-1 T-2]: bob cannot see P-1", gated)
	}
}
