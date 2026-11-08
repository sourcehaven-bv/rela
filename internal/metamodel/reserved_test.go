package metamodel

import (
	"strings"
	"testing"
)

func TestParse_ReservedPrefix(t *testing.T) {
	const base = `
version: "1.0"
entities:
  %ENTITY%:
    label: Task
    id_prefix: "TASK-"
    %ALIASES%
    properties:
      %PROP%:
        type: string
relations:
  %REL%:
    label: blocks
    from: [%ENTITY%]
    to: [%ENTITY%]
    %INVERSE%
    properties:
      %RELPROP%:
        type: string
`
	render := func(repl map[string]string) string {
		s := base
		defaults := map[string]string{
			"%ENTITY%": "task", "%ALIASES%": "", "%PROP%": "title", "%REL%": "blocks",
			"%INVERSE%": "", "%RELPROP%": "note",
		}
		for k, v := range defaults {
			if r, ok := repl[k]; ok {
				v = r
			}
			s = strings.ReplaceAll(s, k, v)
		}
		return s
	}
	tests := []struct {
		name string
		repl map[string]string
		want string
	}{
		{"entity type", map[string]string{"%ENTITY%": "_task"}, `entity type "_task"`},
		{"alias", map[string]string{"%ALIASES%": "aliases: [_t]"}, `alias "_t"`},
		{"entity property", map[string]string{"%PROP%": "_title"}, `property "_title"`},
		{"relation type", map[string]string{"%REL%": "_blocks"}, `relation type "_blocks"`},
		{"inverse", map[string]string{"%INVERSE%": "inverse: { id: _blocked-by }"}, `inverse "_blocked-by"`},
		{"relation property", map[string]string{"%RELPROP%": "_note"}, `property "_note"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(render(tt.repl)))
			if err == nil {
				t.Fatal("expected a load error for a reserved name")
			}
			if !strings.Contains(err.Error(), tt.want) || !strings.Contains(err.Error(), "reserved for rela") {
				t.Errorf("error %q does not name %s as reserved", err.Error(), tt.want)
			}
		})
	}
	t.Run("plain names load", func(t *testing.T) {
		if _, err := Parse([]byte(render(nil))); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	t.Run("underscore inside a name is allowed", func(t *testing.T) {
		repl := map[string]string{"%PROP%": "due_date", "%REL%": "is_blocking"}
		if _, err := Parse([]byte(render(repl))); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestIsReservedName(t *testing.T) {
	for name, want := range map[string]bool{
		"_flow-run": true, "isms._run": true, "task": false, "isms.task": false, "a_b": false,
	} {
		if got := IsReservedName(name); got != want {
			t.Errorf("IsReservedName(%q) = %v; want %v", name, got, want)
		}
	}
}

func TestValidateManagedReferences(t *testing.T) {
	tests := []struct {
		name string
		m    Metamodel
		want string
	}{
		{"on.entity", Metamodel{Automations: []AutomationDef{{
			Name: "a", On: AutomationTrigger{Entity: StringOrSlice{"_flow-run"}},
		}}}, "on.entity"},
		{"on.relation_created", Metamodel{Automations: []AutomationDef{{
			Name: "a", On: AutomationTrigger{RelationCreated: "_carries"},
		}}}, "on.relation_created"},
		{"on.relation_removed", Metamodel{Automations: []AutomationDef{{
			Name: "a", On: AutomationTrigger{RelationRemoved: "_produced"},
		}}}, "on.relation_removed"},
		{"create_relation", Metamodel{Automations: []AutomationDef{{
			Name: "a", Do: []AutomationAction{{CreateRelation: &CreateRelationAction{Relation: "_carries"}}},
		}}}, "create_relation.relation"},
		{"create_entity type", Metamodel{Automations: []AutomationDef{{
			Name: "a", Do: []AutomationAction{{CreateEntity: &CreateEntityAction{Type: "_flow-run"}}},
		}}}, "create_entity.type"},
		{"create_entity relation", Metamodel{Automations: []AutomationDef{{
			Name: "a", Do: []AutomationAction{{CreateEntity: &CreateEntityAction{Type: "task", Relation: "_carries"}}},
		}}}, "create_entity.relation"},
		{"validation entity_type", Metamodel{Validations: []ValidationRule{{
			Name: "v", EntityType: "_flow-run",
		}}}, "entity_type"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := validateManagedReferences(&tt.m)
			if len(errs) != 1 || !strings.Contains(errs[0], tt.want) || !strings.Contains(errs[0], "managed by rela") {
				t.Fatalf("errs = %v, want one error naming %s", errs, tt.want)
			}
		})
	}
	t.Run("plain names pass", func(t *testing.T) {
		m := Metamodel{
			Automations: []AutomationDef{{
				Name: "a",
				On:   AutomationTrigger{Entity: StringOrSlice{"task"}, RelationCreated: "blocks"},
				Do:   []AutomationAction{{CreateEntity: &CreateEntityAction{Type: "task", Relation: "blocks"}}},
			}},
			Validations: []ValidationRule{{Name: "v", EntityType: "task"}},
		}
		if errs := validateManagedReferences(&m); len(errs) != 0 {
			t.Fatalf("unexpected errors: %v", errs)
		}
	})
}

func TestValidationRule_AppliesToType_SkipsManaged(t *testing.T) {
	untyped := ValidationRule{Name: "v"}
	if untyped.AppliesToType("_flow-run") {
		t.Error("an untyped rule applies to a flow run")
	}
	if !untyped.AppliesToType("task") {
		t.Error("an untyped rule does not apply to an operator type")
	}
	typed := ValidationRule{Name: "v", EntityType: "task"}
	if typed.AppliesToType("_flow-run") || !typed.AppliesToType("task") {
		t.Error("a typed rule matches the wrong types")
	}
}
