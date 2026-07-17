package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/output"
)

// whoCanMeta declares the ISMS-style schema the who-can tests exercise.
func whoCanMeta() *metamodel.Metamodel {
	sp := map[string]metamodel.PropertyDef{}
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"person":   {Label: "Person", IDPrefix: "PERS-", Properties: sp},
			"team":     {Label: "Team", IDPrefix: "ROLE-", Properties: sp},
			"folder":   {Label: "Folder", IDPrefix: "FOLDER-", Properties: sp},
			"incident": {Label: "Incident", IDPrefix: "INC-", Properties: sp},
		},
		Relations: map[string]metamodel.RelationDef{
			"member-of":   {From: []string{"person"}, To: []string{"team"}},
			"responds-to": {From: []string{"person"}, To: []string{"incident"}},
			"editor-of":   {From: []string{"person", "team"}, To: []string{"folder", "incident"}},
			"belongs-to":  {From: []string{"incident"}, To: []string{"folder"}},
		},
	}
}

const whoCanPolicy = `
roles:
  everyone:  { read: [folder] }
  responder: { read: [incident], update: [incident] }
  editor:    { read: [incident], create: [incident], update: [incident], delete: [incident] }
  security:  { read: [incident], create: [incident], update: [incident], delete: [incident] }
assignments:
  PERS-ALICE:    security
  ROLE-SECURITY: security
role_relations:
  editor-of:   { confers: editor }
  responds-to: { confers: responder }
inherit_roles_through: [belongs-to]
`

// seedWhoCanGraph writes the canonical scenario into the svc store:
// Alice (global security), Bob (member of ROLE-SECURITY), Carol
// (responds-to INC-042), Dave (editor-of FOLDER-Q3 ⊃ INC-042).
func seedWhoCanGraph(t *testing.T, svc *readServices) {
	t.Helper()
	ctx := context.Background()
	st := svc.Store
	ents := []struct{ id, typ string }{
		{"PERS-ALICE", "person"}, {"PERS-BOB", "person"}, {"PERS-CAROL", "person"}, {"PERS-DAVE", "person"},
		{"ROLE-SECURITY", "team"}, {"FOLDER-Q3", "folder"}, {"INC-042", "incident"},
	}
	for _, e := range ents {
		if err := st.CreateEntity(ctx, entity.New(e.id, e.typ)); err != nil {
			t.Fatalf("seed entity %s: %v", e.id, err)
		}
	}
	rels := []struct{ from, typ, to string }{
		{"PERS-BOB", "member-of", "ROLE-SECURITY"},
		{"PERS-CAROL", "responds-to", "INC-042"},
		{"PERS-DAVE", "editor-of", "FOLDER-Q3"},
		{"INC-042", "belongs-to", "FOLDER-Q3"},
	}
	for _, r := range rels {
		if _, err := st.CreateRelation(ctx, r.from, r.typ, r.to, nil); err != nil {
			t.Fatalf("seed relation %s--%s-->%s: %v", r.from, r.typ, r.to, err)
		}
	}
}

func TestACLWhoCan_ReadTextOutput(t *testing.T) {
	svc := aclTestServices(t, whoCanMeta(), whoCanPolicy)
	seedWhoCanGraph(t, svc)
	buf := withOutput(t, output.FormatTable)

	cmd := &ACLWhoCanCmd{Verb: "read", Entity: "INC-042"}
	if err := cmd.Run(context.Background(), svc); err != nil {
		t.Fatalf("who-can read: %v", err)
	}
	got := buf.String()
	// All four readers, each via its distinct route, must appear.
	for _, want := range []string{
		"PERS-ALICE", "PERS-BOB", "PERS-CAROL", "PERS-DAVE",
		"group ROLE-SECURITY", "responds-to edge", "ancestor FOLDER-Q3",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, got)
		}
	}
}

func TestACLWhoCan_MissingEntityErrors(t *testing.T) {
	svc := aclTestServices(t, whoCanMeta(), whoCanPolicy)
	seedWhoCanGraph(t, svc)
	withOutput(t, output.FormatTable)

	cmd := &ACLWhoCanCmd{Verb: "read", Entity: "INC-NOPE"}
	err := cmd.Run(context.Background(), svc)
	if err == nil {
		t.Fatal("expected an error for a missing entity, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found', got %v", err)
	}
}

func TestACLWhoCan_JSONOutput(t *testing.T) {
	svc := aclTestServices(t, whoCanMeta(), whoCanPolicy)
	seedWhoCanGraph(t, svc)
	buf := withOutput(t, output.FormatJSON)

	cmd := &ACLWhoCanCmd{Verb: "delete", Entity: "INC-042"}
	if err := cmd.Run(context.Background(), svc); err != nil {
		t.Fatalf("who-can delete: %v", err)
	}
	var result struct {
		SchemaVersion int    `json:"schema_version"`
		Verb          string `json:"verb"`
		Entity        string `json:"entity"`
		Principals    []struct {
			Principal string `json:"principal"`
		} `json:"principals"`
	}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal who-can JSON: %v\nraw: %s", err, buf.String())
	}
	if result.SchemaVersion != 1 {
		t.Errorf("schema_version = %d, want 1", result.SchemaVersion)
	}
	if result.Verb != "delete" || result.Entity != "INC-042" {
		t.Errorf("verb/entity = %q/%q, want delete/INC-042", result.Verb, result.Entity)
	}
	// Carol (responder) can update but NOT delete; she must be absent.
	for _, p := range result.Principals {
		if p.Principal == "PERS-CAROL" {
			t.Errorf("PERS-CAROL (responder) must not appear in delete-INC-042 principals")
		}
	}
}

func TestACLWhoCan_NoPolicyFile(t *testing.T) {
	svc := aclTestServices(t, whoCanMeta(), "") // no acl.yaml
	buf := withOutput(t, output.FormatTable)

	cmd := &ACLWhoCanCmd{Verb: "read", Entity: "INC-042"}
	if err := cmd.Run(context.Background(), svc); err != nil {
		t.Fatalf("missing acl.yaml must not error, got %v", err)
	}
	if got := buf.String(); !strings.Contains(got, "no policy") {
		t.Errorf("expected a 'no policy' note, got: %s", got)
	}
}
