package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/output"
)

func TestACLMap_GlobalBaselineText(t *testing.T) {
	svc := aclTestServices(t, whoCanMeta(), whoCanPolicy)
	seedWhoCanGraph(t, svc)
	buf := withOutput(t, output.FormatTable)

	// PERS-ALICE is global security: incident access as a type baseline.
	cmd := &ACLMapCmd{Principal: "PERS-ALICE"}
	if err := cmd.Run(context.Background(), svc); err != nil {
		t.Fatalf("map: %v", err)
	}
	got := buf.String()
	for _, want := range []string{"PERS-ALICE", "incident", "all incident", "[global]"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, got)
		}
	}
}

func TestACLMap_InheritanceExceptionText(t *testing.T) {
	svc := aclTestServices(t, whoCanMeta(), whoCanPolicy)
	seedWhoCanGraph(t, svc)
	buf := withOutput(t, output.FormatTable)

	// PERS-DAVE reaches INC-042 only via folder inheritance — a per-entity
	// exception, not a type baseline.
	cmd := &ACLMapCmd{Principal: "PERS-DAVE"}
	if err := cmd.Run(context.Background(), svc); err != nil {
		t.Fatalf("map: %v", err)
	}
	got := buf.String()
	for _, want := range []string{"exception INC-042", "ancestor FOLDER-Q3", "local-via-ancestor"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, got)
		}
	}
}

func TestACLMap_CutOffPrincipal(t *testing.T) {
	svc := aclTestServices(t, whoCanMeta(), whoCanPolicy)
	seedWhoCanGraph(t, svc)
	buf := withOutput(t, output.FormatTable)

	// A principal with no assignment/group/edge is fully cut off.
	cmd := &ACLMapCmd{Principal: "PERS-GHOST"}
	if err := cmd.Run(context.Background(), svc); err != nil {
		t.Fatalf("map: %v", err)
	}
	if got := buf.String(); !strings.Contains(got, "everyone baseline") {
		t.Errorf("expected a cut-off note for a principal with no personal grant, got:\n%s", got)
	}
}

func TestACLMap_JSONSchema(t *testing.T) {
	svc := aclTestServices(t, whoCanMeta(), whoCanPolicy)
	seedWhoCanGraph(t, svc)
	buf := withOutput(t, output.FormatJSON)

	cmd := &ACLMapCmd{Principal: "PERS-DAVE"}
	if err := cmd.Run(context.Background(), svc); err != nil {
		t.Fatalf("map: %v", err)
	}
	var result struct {
		SchemaVersion int      `json:"schema_version"`
		Principal     string   `json:"principal"`
		Verbs         []string `json:"verbs"`
		EveryoneOnly  bool     `json:"everyone_only"`
		Types         []struct {
			Type       string `json:"type"`
			Exceptions []struct {
				Entity string `json:"entity"`
			} `json:"exceptions"`
		} `json:"types"`
	}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal map JSON: %v\nraw: %s", err, buf.String())
	}
	if result.SchemaVersion != 1 {
		t.Errorf("schema_version = %d, want 1", result.SchemaVersion)
	}
	if result.Principal != "PERS-DAVE" {
		t.Errorf("principal = %q, want PERS-DAVE", result.Principal)
	}
}

func TestACLMap_WholeGraphText(t *testing.T) {
	svc := aclTestServices(t, whoCanMeta(), whoCanPolicy)
	seedWhoCanGraph(t, svc)
	buf := withOutput(t, output.FormatTable)

	// No --principal → whole-graph inventory.
	cmd := &ACLMapCmd{}
	if err := cmd.Run(context.Background(), svc); err != nil {
		t.Fatalf("whole-graph map: %v", err)
	}
	got := buf.String()
	for _, want := range []string{"Whole-graph access", "principal(s)", "PERS-ALICE", "PERS-BOB"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, got)
		}
	}
}

func TestACLMap_WholeGraphJSON(t *testing.T) {
	svc := aclTestServices(t, whoCanMeta(), whoCanPolicy)
	seedWhoCanGraph(t, svc)
	buf := withOutput(t, output.FormatJSON)

	cmd := &ACLMapCmd{}
	if err := cmd.Run(context.Background(), svc); err != nil {
		t.Fatalf("whole-graph map json: %v", err)
	}
	var result struct {
		SchemaVersion  int `json:"schema_version"`
		PrincipalCount int `json:"principal_count"`
		Principals     []struct {
			Principal string `json:"principal"`
		} `json:"principals"`
	}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, buf.String())
	}
	if result.SchemaVersion != 1 {
		t.Errorf("schema_version = %d, want 1", result.SchemaVersion)
	}
	if result.PrincipalCount != len(result.Principals) {
		t.Errorf("principal_count %d != len(principals) %d", result.PrincipalCount, len(result.Principals))
	}
	if result.PrincipalCount == 0 {
		t.Errorf("whole-graph map should enumerate principals")
	}
}

func TestACLMap_NoPolicyFile(t *testing.T) {
	svc := aclTestServices(t, whoCanMeta(), "") // no acl.yaml
	buf := withOutput(t, output.FormatTable)

	cmd := &ACLMapCmd{Principal: "PERS-ALICE"}
	if err := cmd.Run(context.Background(), svc); err != nil {
		t.Fatalf("missing acl.yaml must not error, got %v", err)
	}
	if got := buf.String(); !strings.Contains(got, "no policy") {
		t.Errorf("expected a 'no policy' note, got: %s", got)
	}
}
