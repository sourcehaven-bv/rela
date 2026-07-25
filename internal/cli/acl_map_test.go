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
