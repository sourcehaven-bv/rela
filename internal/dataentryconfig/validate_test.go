package dataentryconfig

import (
	"errors"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// testMetamodel returns a metamodel for testing
func testMetamodel() *metamodel.Metamodel {
	m := &metamodel.Metamodel{
		Version: "1.0",
		Types: map[string]metamodel.CustomType{
			"status": {
				Values:  []string{"open", "in-progress", "closed"},
				Default: "open",
			},
			"priority": {
				Values: []string{"high", "medium", "low"},
			},
		},
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Label:    "Ticket",
				IDPrefix: "TKT-",
				Properties: map[string]metamodel.PropertyDef{
					"title":    {Type: "string", Required: true},
					"status":   {Type: "status"},
					"priority": {Type: "priority"},
					"assignee": {Type: "string"},
				},
			},
			"category": {
				Label:    "Category",
				IDPrefix: "CAT-",
				Properties: map[string]metamodel.PropertyDef{
					"name":        {Type: "string", Required: true},
					"description": {Type: "string"},
				},
			},
		},
		Relations: map[string]metamodel.RelationDef{
			"belongs-to": {
				Label: "belongs to",
				From:  []string{"ticket"},
				To:    []string{"category"},
			},
			"blocks": {
				Label: "blocks",
				From:  []string{"ticket"},
				To:    []string{"ticket"},
			},
		},
	}
	return m
}

func TestValidateConfig_ValidConfig(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Version: "1.0",
		App:     AppConfig{Name: "Test App"},
		Forms: map[string]Form{
			"create_ticket": {
				EntityType: "ticket",
				Title:      "New Ticket",
				Fields: []FormField{
					{Property: "title"},
					{Property: "status"},
				},
				Relations: []FormRelation{
					{Relation: "belongs-to", Direction: "outgoing"},
				},
			},
		},
		Lists: map[string]List{
			"all_tickets": {
				EntityType: "ticket",
				Title:      "All Tickets",
				Columns: []ListColumn{
					{Property: "title"},
					{Property: "status"},
				},
				CreateForm: "create_ticket",
				DetailView: "ticket_view",
			},
		},
		Views: map[string]ViewConfig{
			"ticket_view": {
				Title: "Ticket View",
				Entry: ViewEntry{Type: "ticket"},
				Traverse: []ViewTraverse{
					{From: "entry", Follow: "blocks", CollectAs: "blocked"},
				},
				Sections: []ViewSection{
					{Source: "entry", Display: "properties"},
					{Source: "blocked", Display: "table"},
				},
			},
		},
		Navigation: []NavigationEntry{
			{Label: "Tickets", List: "all_tickets"},
		},
	}

	data := []byte(`version: "1.0"`)

	err := ValidateConfig(data, cfg, meta)
	if err != nil {
		t.Errorf("expected valid config to pass, got error: %v", err)
	}
}

func TestValidateConfig_UnknownTopLevelKey(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{}

	data := []byte(`
version: "1.0"
form:
  test: {}
`)

	err := ValidateConfig(data, cfg, meta)
	if err == nil {
		t.Fatal("expected error for unknown key 'form'")
	}
	if !strings.Contains(err.Error(), `unknown key "form"`) {
		t.Errorf("expected error about unknown key, got: %v", err)
	}
	if !strings.Contains(err.Error(), `did you mean "forms"`) {
		t.Errorf("expected suggestion for 'forms', got: %v", err)
	}
}

func TestValidateConfig_UnknownFormEntityType(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Forms: map[string]Form{
			"test": {EntityType: "unknown_type"},
		},
	}

	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil {
		t.Fatal("expected error for unknown entity type")
	}
	if !strings.Contains(err.Error(), `form "test": unknown entity type "unknown_type"`) {
		t.Errorf("expected error about unknown entity type, got: %v", err)
	}
}

func TestValidateConfig_UnknownFormFieldProperty(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Forms: map[string]Form{
			"test": {
				EntityType: "ticket",
				Fields:     []FormField{{Property: "unknown_prop"}},
			},
		},
	}

	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil {
		t.Fatal("expected error for unknown property")
	}
	if !strings.Contains(err.Error(), `property "unknown_prop" not in metamodel`) {
		t.Errorf("expected error about unknown property, got: %v", err)
	}
}

func TestValidateConfig_InvalidTransitions(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Forms: map[string]Form{
			"test": {
				EntityType: "ticket",
				Fields: []FormField{{
					Property: "status",
					Transitions: map[string][]string{
						"invalid_state": {"open"},
						"open":          {"invalid_target"},
					},
				}},
			},
		},
	}

	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil {
		t.Fatal("expected error for invalid transitions")
	}
	if !strings.Contains(err.Error(), `invalid from-state "invalid_state"`) {
		t.Errorf("expected error about invalid from-state, got: %v", err)
	}
	if !strings.Contains(err.Error(), `invalid to-state "invalid_target"`) {
		t.Errorf("expected error about invalid to-state, got: %v", err)
	}
}

func TestValidateConfig_UnknownFormRelation(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Forms: map[string]Form{
			"test": {
				EntityType: "ticket",
				Relations:  []FormRelation{{Relation: "unknown_rel"}},
			},
		},
	}

	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil {
		t.Fatal("expected error for unknown relation")
	}
	if !strings.Contains(err.Error(), `unknown relation "unknown_rel"`) {
		t.Errorf("expected error about unknown relation, got: %v", err)
	}
}

func TestValidateConfig_InvalidRelationDirection(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Forms: map[string]Form{
			"test": {
				EntityType: "ticket",
				Relations:  []FormRelation{{Relation: "blocks", Direction: "both"}},
			},
		},
	}

	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil {
		t.Fatal("expected error for invalid direction")
	}
	if !strings.Contains(err.Error(), `invalid direction "both"`) {
		t.Errorf("expected error about invalid direction, got: %v", err)
	}
}

func TestValidateConfig_InvalidRelationWidget(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Forms: map[string]Form{
			"test": {
				EntityType: "ticket",
				Relations:  []FormRelation{{Relation: "blocks", Widget: "banana"}},
			},
		},
	}

	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil {
		t.Fatal("expected error for invalid widget")
	}
	if !strings.Contains(err.Error(), `invalid widget "banana"`) {
		t.Errorf("expected error about invalid widget, got: %v", err)
	}
}

func TestValidateConfig_FormRelationUnknownCreateForm(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Forms: map[string]Form{
			"test": {
				EntityType: "ticket",
				Relations:  []FormRelation{{Relation: "blocks", CreateForm: "nonexistent"}},
			},
		},
	}

	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil {
		t.Fatal("expected error for unknown create_form")
	}
	if !strings.Contains(err.Error(), `unknown create_form "nonexistent"`) {
		t.Errorf("expected error about unknown create_form, got: %v", err)
	}
}

func TestValidateConfig_UnknownListEntityType(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Lists: map[string]List{
			"test": {EntityType: "unknown_type"},
		},
	}

	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil {
		t.Fatal("expected error for unknown entity type")
	}
	if !strings.Contains(err.Error(), `list "test": unknown entity type "unknown_type"`) {
		t.Errorf("expected error about unknown entity type, got: %v", err)
	}
}

func TestValidateConfig_UnknownListColumnProperty(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Lists: map[string]List{
			"test": {
				EntityType: "ticket",
				Columns:    []ListColumn{{Property: "unknown_prop"}},
			},
		},
	}

	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil {
		t.Fatal("expected error for unknown column property")
	}
	if !strings.Contains(err.Error(), `column[0] property "unknown_prop" not in metamodel`) {
		t.Errorf("expected error about unknown column property, got: %v", err)
	}
}

func TestValidateConfig_UnknownListColumnRelation(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Lists: map[string]List{
			"test": {
				EntityType: "ticket",
				Columns:    []ListColumn{{Relation: "unknown_rel"}},
			},
		},
	}

	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil {
		t.Fatal("expected error for unknown column relation")
	}
	if !strings.Contains(err.Error(), `column[0] references unknown relation "unknown_rel"`) {
		t.Errorf("expected error about unknown column relation, got: %v", err)
	}
}

func TestValidateConfig_InvalidSortDirection(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Lists: map[string]List{
			"test": {
				EntityType: "ticket",
				Sort:       []SortSpec{{Property: "title", Direction: "up"}},
			},
		},
	}

	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil {
		t.Fatal("expected error for invalid sort direction")
	}
	if !strings.Contains(err.Error(), `invalid direction "up"`) {
		t.Errorf("expected error about invalid direction, got: %v", err)
	}
}

func TestValidateConfig_InvalidFilterOperator(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Lists: map[string]List{
			"test": {
				EntityType: "ticket",
				Filters:    []FilterConfig{{Property: "status", Operator: "=="}},
			},
		},
	}

	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil {
		t.Fatal("expected error for invalid filter operator")
	}
	if !strings.Contains(err.Error(), `invalid operator "=="`) {
		t.Errorf("expected error about invalid operator, got: %v", err)
	}
}

func TestValidateConfig_ListReferencesUnknownForm(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Forms: map[string]Form{
			"create_ticket": {EntityType: "ticket"},
		},
		Lists: map[string]List{
			"test": {
				EntityType: "ticket",
				CreateForm: "nonexistent_form",
			},
		},
	}

	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil {
		t.Fatal("expected error for unknown create_form")
	}
	if !strings.Contains(err.Error(), `references unknown form "nonexistent_form"`) {
		t.Errorf("expected error about unknown form, got: %v", err)
	}
}

func TestValidateConfig_ListReferencesUnknownView(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Views: map[string]ViewConfig{
			"ticket_view": {Entry: ViewEntry{Type: "ticket"}},
		},
		Lists: map[string]List{
			"test": {
				EntityType: "ticket",
				DetailView: "nonexistent_view",
			},
		},
	}

	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil {
		t.Fatal("expected error for unknown detail_view")
	}
	if !strings.Contains(err.Error(), `references unknown view "nonexistent_view"`) {
		t.Errorf("expected error about unknown view, got: %v", err)
	}
}

func TestValidateConfig_ViewUnknownEntryType(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Views: map[string]ViewConfig{
			"test": {Entry: ViewEntry{Type: "unknown_type"}},
		},
	}

	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil {
		t.Fatal("expected error for unknown entry type")
	}
	if !strings.Contains(err.Error(), `entry type "unknown_type" not in metamodel`) {
		t.Errorf("expected error about unknown entry type, got: %v", err)
	}
}

func TestValidateConfig_ViewTraverseUnknownCollection(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Views: map[string]ViewConfig{
			"test": {
				Entry: ViewEntry{Type: "ticket"},
				Traverse: []ViewTraverse{
					{From: "nonexistent", Follow: "blocks", CollectAs: "blocked"},
				},
			},
		},
	}

	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil {
		t.Fatal("expected error for unknown collection in from")
	}
	if !strings.Contains(err.Error(), `unknown collection "nonexistent" in from`) {
		t.Errorf("expected error about unknown collection, got: %v", err)
	}
}

func TestValidateConfig_ViewTraverseUnknownRelation(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Views: map[string]ViewConfig{
			"test": {
				Entry: ViewEntry{Type: "ticket"},
				Traverse: []ViewTraverse{
					{From: "entry", Follow: "block", CollectAs: "blocked"}, // typo: block vs blocks
				},
			},
		},
	}

	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil {
		t.Fatal("expected error for unknown relation")
	}
	if !strings.Contains(err.Error(), `unknown relation "block"`) {
		t.Errorf("expected error about unknown relation, got: %v", err)
	}
	if !strings.Contains(err.Error(), `did you mean "blocks"`) {
		t.Errorf("expected suggestion, got: %v", err)
	}
}

func TestValidateConfig_ViewTraverseMissingFollowOrFollowIncoming(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Views: map[string]ViewConfig{
			"test": {
				Entry: ViewEntry{Type: "ticket"},
				Traverse: []ViewTraverse{
					{From: "entry", CollectAs: "blocked"},
				},
			},
		},
	}

	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil {
		t.Fatal("expected error for missing follow/follow_incoming")
	}
	if !strings.Contains(err.Error(), "must specify either follow or follow_incoming") {
		t.Errorf("expected error about missing follow, got: %v", err)
	}
}

func TestValidateConfig_ViewTraverseBothFollowAndFollowIncoming(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Views: map[string]ViewConfig{
			"test": {
				Entry: ViewEntry{Type: "ticket"},
				Traverse: []ViewTraverse{
					{From: "entry", Follow: "blocks", FollowIncoming: "blocks", CollectAs: "blocked"},
				},
			},
		},
	}

	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil {
		t.Fatal("expected error for both follow and follow_incoming")
	}
	if !strings.Contains(err.Error(), "cannot specify both follow and follow_incoming") {
		t.Errorf("expected error about both follow, got: %v", err)
	}
}

func TestValidateConfig_ViewTraverseMissingCollectAs(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Views: map[string]ViewConfig{
			"test": {
				Entry: ViewEntry{Type: "ticket"},
				Traverse: []ViewTraverse{
					{From: "entry", Follow: "blocks"},
				},
			},
		},
	}

	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil {
		t.Fatal("expected error for missing collect_as")
	}
	if !strings.Contains(err.Error(), "must specify collect_as") {
		t.Errorf("expected error about missing collect_as, got: %v", err)
	}
}

func TestValidateConfig_ViewSectionUnknownSource(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Views: map[string]ViewConfig{
			"test": {
				Entry: ViewEntry{Type: "ticket"},
				Sections: []ViewSection{
					{Source: "nonexistent", Display: "table"},
				},
			},
		},
	}

	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil {
		t.Fatal("expected error for unknown section source")
	}
	if !strings.Contains(err.Error(), `unknown collection "nonexistent" in source`) {
		t.Errorf("expected error about unknown source, got: %v", err)
	}
}

func TestValidateConfig_ViewSectionInvalidDisplay(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Views: map[string]ViewConfig{
			"test": {
				Entry: ViewEntry{Type: "ticket"},
				Sections: []ViewSection{
					{Source: "entry", Display: "grid"},
				},
			},
		},
	}

	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil {
		t.Fatal("expected error for invalid display mode")
	}
	if !strings.Contains(err.Error(), `invalid display mode "grid"`) {
		t.Errorf("expected error about invalid display, got: %v", err)
	}
}

func TestValidateConfig_ViewSectionUnknownFieldProperty(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Views: map[string]ViewConfig{
			"test": {
				Entry: ViewEntry{Type: "ticket"},
				Sections: []ViewSection{
					{Source: "entry", Display: "properties", Fields: []ViewSectionField{{Property: "unknown_prop"}}},
				},
			},
		},
	}

	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil {
		t.Fatal("expected error for unknown field property")
	}
	if !strings.Contains(err.Error(), `property "unknown_prop" not in entity`) {
		t.Errorf("expected error about unknown property, got: %v", err)
	}
}

func TestValidateConfig_DashboardInvalidDisplay(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Dashboard: &DashboardConfig{
			Cards: []DashboardCard{
				{Title: "Test", Display: "pie"},
			},
		},
	}

	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil {
		t.Fatal("expected error for invalid dashboard display")
	}
	if !strings.Contains(err.Error(), `invalid display mode "pie"`) {
		t.Errorf("expected error about invalid display, got: %v", err)
	}
}

func TestValidateConfig_DashboardBreakdownMissingGroupBy(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Dashboard: &DashboardConfig{
			Cards: []DashboardCard{
				{Title: "Test", Display: "breakdown"},
			},
		},
	}

	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil {
		t.Fatal("expected error for breakdown without group_by")
	}
	if !strings.Contains(err.Error(), "uses breakdown display but has no group_by") {
		t.Errorf("expected error about missing group_by, got: %v", err)
	}
}

func TestValidateConfig_DashboardTableMissingColumns(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Dashboard: &DashboardConfig{
			Cards: []DashboardCard{
				{Title: "Test", Display: "table"},
			},
		},
	}

	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil {
		t.Fatal("expected error for table without columns")
	}
	if !strings.Contains(err.Error(), "uses table display but has no columns") {
		t.Errorf("expected error about missing columns, got: %v", err)
	}
}

func TestValidateConfig_CommandMissingLabel(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Commands: map[string]CommandConfig{
			"test": {Script: "echo hello", Context: "global"},
		},
	}

	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil {
		t.Fatal("expected error for missing label")
	}
	if !strings.Contains(err.Error(), `command "test": label is required`) {
		t.Errorf("expected error about missing label, got: %v", err)
	}
}

func TestValidateConfig_CommandMissingScript(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Commands: map[string]CommandConfig{
			"test": {Label: "Test", Context: "global"},
		},
	}

	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil {
		t.Fatal("expected error for missing script")
	}
	if !strings.Contains(err.Error(), `command "test": script is required`) {
		t.Errorf("expected error about missing script, got: %v", err)
	}
}

func TestValidateConfig_CommandInvalidContext(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Commands: map[string]CommandConfig{
			"test": {Label: "Test", Script: "echo", Context: "anywhere"},
		},
	}

	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil {
		t.Fatal("expected error for invalid context")
	}
	if !strings.Contains(err.Error(), `invalid context "anywhere"`) {
		t.Errorf("expected error about invalid context, got: %v", err)
	}
}

func TestValidateConfig_CommandUnknownAvailableOnView(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Commands: map[string]CommandConfig{
			"test": {
				Label:       "Test",
				Script:      "echo",
				Context:     "view",
				AvailableOn: &CommandScope{Views: []string{"unknown_view"}},
			},
		},
	}

	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil {
		t.Fatal("expected error for unknown view")
	}
	if !strings.Contains(err.Error(), `references unknown view "unknown_view"`) {
		t.Errorf("expected error about unknown view, got: %v", err)
	}
}

func TestValidateConfig_StyleUnknownType(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Styles: map[string]map[string]string{
			"unknown_type": {"value": "blue"},
		},
	}

	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil {
		t.Fatal("expected error for unknown style type")
	}
	if !strings.Contains(err.Error(), `type "unknown_type" is not defined in metamodel`) {
		t.Errorf("expected error about unknown type, got: %v", err)
	}
}

func TestValidateConfig_StyleValidCustomType(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Styles: map[string]map[string]string{
			"status":   {"open": "blue"},
			"priority": {"high": "red"},
		},
	}

	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err != nil {
		t.Errorf("expected valid styles to pass, got: %v", err)
	}
}

func TestValidateConfig_NavigationUnknownList(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Navigation: []NavigationEntry{
			{Label: "Test", List: "unknown_list"},
		},
	}

	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil {
		t.Fatal("expected error for unknown list in navigation")
	}
	if !strings.Contains(err.Error(), `references unknown list "unknown_list"`) {
		t.Errorf("expected error about unknown list, got: %v", err)
	}
}

func TestValidateConfig_NavigationNestedGroups(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Navigation: []NavigationEntry{
			{
				Group: "Outer",
				Items: []NavigationEntry{
					{Group: "Inner"},
				},
			},
		},
	}

	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil {
		t.Fatal("expected error for nested groups")
	}
	if !strings.Contains(err.Error(), "nested groups are not supported") {
		t.Errorf("expected error about nested groups, got: %v", err)
	}
}

func TestValidateConfig_MultipleErrors(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Forms: map[string]Form{
			"test": {EntityType: "unknown1"},
		},
		Lists: map[string]List{
			"test": {EntityType: "unknown2"},
		},
	}

	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil {
		t.Fatal("expected multiple errors")
	}

	var configErr *ConfigValidationError
	if !errors.As(err, &configErr) {
		t.Fatalf("expected ConfigValidationError, got %T", err)
	}
	if len(configErr.Errors) != 2 {
		t.Errorf("expected 2 errors, got %d: %v", len(configErr.Errors), configErr.Errors)
	}
}

func TestValidateConfig_ViewTraverseWildcardFrom(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Views: map[string]ViewConfig{
			"test": {
				Entry: ViewEntry{Type: "ticket"},
				Traverse: []ViewTraverse{
					{From: "entry", Follow: "blocks", CollectAs: "blocked"},
					{From: "*", Follow: "belongs-to", CollectAs: "categories"},
				},
				Sections: []ViewSection{
					{Source: "categories", Display: "list"},
				},
			},
		},
	}

	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err != nil {
		t.Errorf("expected wildcard from to be valid, got: %v", err)
	}
}

func TestValidateConfig_ViewSectionUsesPreviousCollectAs(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Views: map[string]ViewConfig{
			"test": {
				Entry: ViewEntry{Type: "ticket"},
				Traverse: []ViewTraverse{
					{From: "entry", Follow: "blocks", CollectAs: "blocked"},
				},
				Sections: []ViewSection{
					{Source: "blocked", Display: "table", Columns: []ListColumn{{Property: "title"}}},
				},
			},
		},
	}

	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err != nil {
		t.Errorf("expected section using collect_as to be valid, got: %v", err)
	}
}

func TestValidateConfig_DocumentMissingCommand(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Documents: map[string]DocumentConfig{
			"spec": {View: "spec-view"},
		},
	}

	err := ValidateConfig(nil, cfg, meta)
	if err == nil {
		t.Error("expected error for document with missing command")
	}
	errStr := err.Error()
	if !strings.Contains(errStr, "command is required") {
		t.Errorf("expected 'command is required' error, got: %s", errStr)
	}
}

func TestValidateConfig_DocumentMissingEntityType(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Documents: map[string]DocumentConfig{
			"spec": {Command: "render.sh"},
		},
	}

	err := ValidateConfig(nil, cfg, meta)
	if err == nil {
		t.Error("expected error for document with missing entity_type")
	}
	errStr := err.Error()
	if !strings.Contains(errStr, "entity_type is required") {
		t.Errorf("expected 'entity_type is required' error, got: %s", errStr)
	}
}

func TestValidateConfig_DocumentValid(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Documents: map[string]DocumentConfig{
			"spec": {Command: "render.sh", EntityType: "requirement"},
		},
	}

	err := ValidateConfig(nil, cfg, meta)
	if err != nil {
		t.Errorf("expected valid document config to pass, got: %v", err)
	}
}

func TestValidateConfig_DocumentValidWithView(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Documents: map[string]DocumentConfig{
			"spec": {Command: "render.sh", EntityType: "requirement", View: "spec-view"},
		},
	}

	err := ValidateConfig(nil, cfg, meta)
	if err != nil {
		t.Errorf("expected valid document config with view to pass, got: %v", err)
	}
}

func TestConfigValidationError_SingleError(t *testing.T) {
	err := &ConfigValidationError{Errors: []string{"single error"}}
	if err.Error() != "single error" {
		t.Errorf("expected 'single error', got %q", err.Error())
	}
}

func TestConfigValidationError_MultipleErrors(t *testing.T) {
	err := &ConfigValidationError{Errors: []string{"error 1", "error 2"}}
	expected := "data-entry config validation errors:\n  - error 1\n  - error 2"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}

func TestValidateConfig_FilterControlMissingPropertyAndRelation(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Lists: map[string]List{
			"test": {
				EntityType:     "ticket",
				FilterControls: []FilterControl{{}},
			},
		},
	}

	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil {
		t.Fatal("expected error for filter_control with neither property nor relation")
	}
	if !strings.Contains(err.Error(), "must specify either property or relation") {
		t.Errorf("expected error about missing property or relation, got: %v", err)
	}
}

func TestValidateConfig_FilterControlUnknownRelation(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Lists: map[string]List{
			"test": {
				EntityType:     "ticket",
				FilterControls: []FilterControl{{Relation: "unknown_rel"}},
			},
		},
	}

	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil {
		t.Fatal("expected error for filter_control with unknown relation")
	}
	if !strings.Contains(err.Error(), `references unknown relation "unknown_rel"`) {
		t.Errorf("expected error about unknown relation, got: %v", err)
	}
}

func TestValidateConfig_FilterControlValidRelation(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Lists: map[string]List{
			"test": {
				EntityType:     "ticket",
				FilterControls: []FilterControl{{Relation: "belongs-to"}},
			},
		},
	}

	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err != nil {
		t.Errorf("expected valid filter_control relation to pass, got: %v", err)
	}
}

func TestValidateConfig_KanbanFilterControlMissingPropertyAndRelation(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Kanbans: map[string]Kanban{
			"test": {
				EntityType:     "ticket",
				ColumnProperty: "status",
				FilterControls: []FilterControl{{}},
			},
		},
	}

	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil {
		t.Fatal("expected error for kanban filter_control with neither property nor relation")
	}
	if !strings.Contains(err.Error(), "must specify either property or relation") {
		t.Errorf("expected error about missing property or relation, got: %v", err)
	}
}

func TestValidateConfig_KanbanFilterControlUnknownRelation(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Kanbans: map[string]Kanban{
			"test": {
				EntityType:     "ticket",
				ColumnProperty: "status",
				FilterControls: []FilterControl{{Relation: "unknown_rel"}},
			},
		},
	}

	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil {
		t.Fatal("expected error for kanban filter_control with unknown relation")
	}
	if !strings.Contains(err.Error(), `references unknown relation "unknown_rel"`) {
		t.Errorf("expected error about unknown relation, got: %v", err)
	}
}

func TestValidateActions_ValidConfig(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Actions: map[string]Action{
			"today_note": {Script: "today.lua"},
			"find_or_create": {
				Script: "find-or-create.lua",
				Params: map[string]string{"entity_type": "ticket"},
			},
		},
	}
	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateActions_InvalidID(t *testing.T) {
	meta := testMetamodel()
	tests := []string{"Today_Note", "today note", "today/note", "today..note", "x" + strings.Repeat("y", 64)}
	for _, id := range tests {
		t.Run(id, func(t *testing.T) {
			cfg := &Config{
				Actions: map[string]Action{id: {Script: "x.lua"}},
			}
			err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
			if err == nil {
				t.Fatalf("expected error for invalid ID %q", id)
			}
		})
	}
}

func TestValidateActions_EmptyScript(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Actions: map[string]Action{"foo": {Script: ""}},
	}
	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil || !strings.Contains(err.Error(), "empty script") {
		t.Errorf("expected empty script error, got: %v", err)
	}
}

func TestValidateActions_PathTraversal(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Actions: map[string]Action{"foo": {Script: "../etc/passwd.lua"}},
	}
	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil || !strings.Contains(err.Error(), "local path") {
		t.Errorf("expected path traversal error, got: %v", err)
	}
}

func TestValidateActions_WrongExtension(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Actions: map[string]Action{"foo": {Script: "script.txt"}},
	}
	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil || !strings.Contains(err.Error(), ".lua") {
		t.Errorf("expected extension error, got: %v", err)
	}
}

func TestValidateNavigation_UnknownAction(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Navigation: []NavigationEntry{
			{Label: "Today", Action: "missing_action"},
		},
	}
	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil || !strings.Contains(err.Error(), `unknown action "missing_action"`) {
		t.Errorf("expected unknown action error, got: %v", err)
	}
}

func TestValidateNavigation_KnownAction(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Actions: map[string]Action{
			"today_note": {Script: "today.lua"},
		},
		Navigation: []NavigationEntry{
			{Label: "Today", Action: "today_note"},
		},
	}
	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
