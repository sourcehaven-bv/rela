package datamigration

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

func TestParseStep_RefusesManagedTargets(t *testing.T) {
	tests := []struct {
		name string
		step string
		key  string
	}{
		{"entity", "rename_property: {entity: _flow-run, from: a, to: b}", "entity"},
		{"from", "rename_entity_type: {from: _flow-run, to: run}", "from"},
		{"to", "rename_entity_type: {from: run, to: _flow-run}", "to"},
		{"type", "drop_relations: {type: _carries}", "type"},
		{"to_type", "convert: {entity: task, property: p, to_type: _flow-run}", "to_type"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var doc yaml.Node
			if err := yaml.Unmarshal([]byte(tt.step), &doc); err != nil {
				t.Fatal(err)
			}
			_, err := parseStep(doc.Content[0])
			if err == nil {
				t.Fatal("expected the step to be refused")
			}
			if !strings.Contains(err.Error(), tt.key+":") || !strings.Contains(err.Error(), "managed by rela") {
				t.Errorf("error %q does not name %s as managed", err, tt.key)
			}
		})
	}
	t.Run("plain names parse", func(t *testing.T) {
		var doc yaml.Node
		if err := yaml.Unmarshal([]byte("rename_property: {entity: task, from: a, to: b}"), &doc); err != nil {
			t.Fatal(err)
		}
		if _, err := parseStep(doc.Content[0]); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestForEachEntity_WildcardSkipsManaged(t *testing.T) {
	st := seedStore(t)
	run := &entity.Entity{ID: "RUN-1", Type: entity.FlowRunType, Properties: map[string]any{"title": "run"}}
	if err := st.CreateEntity(t.Context(), run); err != nil {
		t.Fatal(err)
	}
	x := &Exec{Store: st, Apply: true}
	res, err := (&dropPropertyStep{Entity: "*", Property: "title"}).Run(t.Context(), x)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Affected != 3 { // the three tasks, not the run
		t.Errorf("affected %d, want 3", res.Affected)
	}
	if _, has := getEntity(t, st, "RUN-1").Properties["title"]; !has {
		t.Error("a wildcard step changed a flow run")
	}
	if _, has := getEntity(t, st, "TSK-1").Properties["title"]; has {
		t.Error("the wildcard step did not change a task")
	}
}
