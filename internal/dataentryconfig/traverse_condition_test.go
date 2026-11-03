package dataentryconfig

import (
	"slices"
	"strings"
	"testing"
)

// TestValidateConfig_TraverseWhereMustParse pins BUG-WHEREWIDE: an
// unparseable `where:` on a relation step fails the load. It used to be
// dropped at render time, which showed every traversed entity.
func TestValidateConfig_TraverseWhereMustParse(t *testing.T) {
	cases := []struct {
		name, where, wantErr string
	}{
		{"single clause", "status = done", ""},
		{"type clause", "type = ticket", ""},
		{"missing operator", "status", `view "test": traverse[0] has an invalid where`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{Views: map[string]ViewConfig{"test": {
				Entry:    ViewEntry{Type: "ticket"},
				Traverse: []ViewTraverse{{From: "entry", Follow: "blocks", CollectAs: "blocked", Where: tc.where}},
			}}}
			err := ValidateConfig([]byte(`version: "1.0"`), cfg, testMetamodel())
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("expected valid config, got: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("expected error containing %q, got: %v", tc.wantErr, err)
			}
		})
	}
}

// A side panel takes no traverse condition, and its where must parse.
func TestValidateConfig_SidePanelTraverseConditionAndWhere(t *testing.T) {
	cfg := &Config{Forms: map[string]Form{
		"ticket_form": {
			EntityType: "ticket",
			Fields:     []FormField{{Property: "title"}},
			SidePanel: &SidePanelConfig{
				Traverse: []ViewTraverse{
					{From: "entry", Follow: "blocks", CollectAs: "a", Condition: "entity.status == 'done'"},
					{From: "entry", Follow: "blocks", CollectAs: "b", Where: "status"},
				},
			},
		},
	}}
	err := ValidateConfig([]byte(`version: "1.0"`), cfg, testMetamodel())
	if err == nil {
		t.Fatal("expected the side panel to be refused")
	}
	for _, want := range []string{"side_panel traverse[0] sets condition", "side_panel traverse[1] has an invalid where"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error lacks %q: %v", want, err)
		}
	}
}

func TestTraverseConditionID(t *testing.T) {
	a := TraverseConditionID("taak", []string{"taak", "project", "taak"}, "entity.type == 'taak'")
	b := TraverseConditionID("taak", []string{"project", "taak"}, "entity.type == 'taak'")
	if a != b {
		t.Errorf("type order or duplicates changed the id: %q vs %q", a, b)
	}
	if et, ok := SplitTraverseConditionID(a); !ok || et != "taak" {
		t.Errorf("SplitTraverseConditionID = %q, %v; want taak, true", et, ok)
	}
	if TraverseConditionID("project", []string{"project", "taak"}, "x") == TraverseConditionID("project", []string{"project"}, "x") {
		t.Error("the type set is not part of the id")
	}
}

func TestTraverseTargetTypes(t *testing.T) {
	meta := testMetamodel()
	rel := meta.Relations["blocks"]
	if got := TraverseTargetTypes(ViewTraverse{Follow: "blocks"}, meta); !slices.Equal(got, rel.To) {
		t.Errorf("follow: got %v, want %v", got, rel.To)
	}
	if got := TraverseTargetTypes(ViewTraverse{FollowIncoming: "blocks"}, meta); !slices.Equal(got, rel.From) {
		t.Errorf("follow_incoming: got %v, want %v", got, rel.From)
	}
	if got := TraverseTargetTypes(ViewTraverse{MentionedBy: true}, meta); len(got) != len(meta.Entities) {
		t.Errorf("mentioned_by: got %v, want every type", got)
	}
	if got := TraverseTargetTypes(ViewTraverse{Follow: "nope"}, meta); got != nil {
		t.Errorf("unknown relation: got %v, want nil", got)
	}
}
