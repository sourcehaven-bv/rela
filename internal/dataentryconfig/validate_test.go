package dataentryconfig

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// TestValidTopLevelKeysMatchConfigStruct guards the same drift class as
// BUG-5XIN07 (the metamodel loader's whitelist), applied to data-entry.yaml:
// validTopLevelKeys is a hand-maintained duplicate of the Config struct's
// top-level yaml tags, so a newly-added field could be silently rejected by
// checkUnknownKeys. This asserts every top-level yaml tag on Config is
// whitelisted, so such a field fails this test before it can reach a user.
func TestValidTopLevelKeysMatchConfigStruct(t *testing.T) {
	for field := range reflect.TypeFor[Config]().Fields() {
		tag := field.Tag.Get("yaml")
		if tag == "" || tag == "-" {
			continue // computed / non-YAML field
		}
		key := strings.Split(tag, ",")[0]
		if key == "" {
			continue
		}
		if !validTopLevelKeys[key] {
			t.Errorf("Config field %q (yaml:%q) is not in validTopLevelKeys — "+
				"checkUnknownKeys will reject a config that uses it", field.Name, key)
		}
	}
}

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

// inverseTestMetamodel parses a metamodel via the real loader so
// inverseOwners is populated — needed for tests that exercise the
// inverse-name and wrong-side branches in validateForms.
func inverseTestMetamodel(t *testing.T) *metamodel.Metamodel {
	t.Helper()
	yaml := `version: "1.0"
entities:
  from_entity:
    label: From
    id_prefix: "FROM-"
    properties:
      title:
        type: string
        required: true
  to_entity:
    label: To
    id_prefix: "TO-"
    properties:
      title:
        type: string
        required: true
  other_entity:
    label: Other
    id_prefix: "OTH-"
    properties:
      title:
        type: string
        required: true
relations:
  connects_to:
    label: connects to
    from: [from_entity]
    to: [to_entity]
    inverse: connects_from
  multi_source:
    label: multi source
    from: [from_entity, other_entity]
    to: [to_entity]
`
	m, err := metamodel.Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("parse inverse test metamodel: %v", err)
	}
	return m
}

func TestValidateConfig_FormRelationInverseName(t *testing.T) {
	meta := inverseTestMetamodel(t)
	cfg := &Config{
		Forms: map[string]Form{
			"edit_to": {
				EntityType: "to_entity",
				Relations:  []FormRelation{{Relation: "connects_from"}},
			},
		},
	}

	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil {
		t.Fatal("expected error for inverse-name relation")
	}
	msg := err.Error()
	// Error must identify the inverse name and point the user at the
	// canonical relation plus direction: incoming.
	if !strings.Contains(msg, `"connects_from"`) {
		t.Errorf("expected error to mention inverse name, got: %v", err)
	}
	if !strings.Contains(msg, `"connects_to"`) {
		t.Errorf("expected error to point at canonical name, got: %v", err)
	}
	if !strings.Contains(msg, "direction: incoming") {
		t.Errorf("expected error to mention direction: incoming, got: %v", err)
	}
}

func TestValidateConfig_FormRelationWrongSide_HintsIncoming(t *testing.T) {
	meta := inverseTestMetamodel(t)
	// to_entity is on the TO side of connects_to; the default outgoing
	// direction makes this form silently broken.
	cfg := &Config{
		Forms: map[string]Form{
			"edit_to": {
				EntityType: "to_entity",
				Relations:  []FormRelation{{Relation: "connects_to"}},
			},
		},
	}

	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil {
		t.Fatal("expected error for form entity not on the source side of an outgoing relation")
	}
	msg := err.Error()
	if !strings.Contains(msg, `"to_entity"`) {
		t.Errorf("expected error to mention form entity type, got: %v", err)
	}
	if !strings.Contains(msg, `"connects_to"`) {
		t.Errorf("expected error to mention the relation, got: %v", err)
	}
	if !strings.Contains(msg, "direction: incoming") {
		t.Errorf("expected hint to set direction: incoming, got: %v", err)
	}
}

func TestValidateConfig_FormRelationWrongSide_NoHintWhenDirectionExplicit(t *testing.T) {
	meta := inverseTestMetamodel(t)
	// Explicit outgoing with form entity on the wrong side — flipping the
	// direction would help, but the user has explicitly chosen outgoing,
	// so we error without a "did you mean" hint.
	cfg := &Config{
		Forms: map[string]Form{
			"edit_to": {
				EntityType: "to_entity",
				Relations:  []FormRelation{{Relation: "connects_to", Direction: DirectionOutgoing}},
			},
		},
	}

	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil {
		t.Fatal("expected error for form entity not on the source side")
	}
	if !strings.Contains(err.Error(), `"to_entity"`) {
		t.Errorf("expected error to mention form entity type, got: %v", err)
	}
}

func TestValidateConfig_FormRelationIncomingWrongSide(t *testing.T) {
	meta := inverseTestMetamodel(t)
	// from_entity is the SOURCE side; declaring direction: incoming on it
	// for connects_to is a configuration error.
	cfg := &Config{
		Forms: map[string]Form{
			"edit_from": {
				EntityType: "from_entity",
				Relations:  []FormRelation{{Relation: "connects_to", Direction: DirectionIncoming}},
			},
		},
	}

	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil {
		t.Fatal("expected error for direction: incoming on the source side")
	}
	if !strings.Contains(err.Error(), `"from_entity"`) {
		t.Errorf("expected error to mention form entity type, got: %v", err)
	}
}

func TestValidateConfig_FormRelationCorrectSide(t *testing.T) {
	meta := inverseTestMetamodel(t)
	// from_entity → connects_to (outgoing default) is the correct shape.
	// to_entity   → connects_to with direction: incoming is also correct.
	cfg := &Config{
		Forms: map[string]Form{
			"edit_from": {
				EntityType: "from_entity",
				Relations:  []FormRelation{{Relation: "connects_to"}},
			},
			"edit_to": {
				EntityType: "to_entity",
				Relations:  []FormRelation{{Relation: "connects_to", Direction: DirectionIncoming}},
			},
		},
	}

	if err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta); err != nil {
		t.Errorf("expected valid config to pass, got: %v", err)
	}
}

func TestValidateConfig_FormRelationMultiSourceSide(t *testing.T) {
	meta := inverseTestMetamodel(t)
	// multi_source allows from_entity OR other_entity as the source.
	// Both should pass when used as the form entity with outgoing default.
	cfg := &Config{
		Forms: map[string]Form{
			"edit_from": {
				EntityType: "from_entity",
				Relations:  []FormRelation{{Relation: "multi_source"}},
			},
			"edit_other": {
				EntityType: "other_entity",
				Relations:  []FormRelation{{Relation: "multi_source"}},
			},
		},
	}

	if err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta); err != nil {
		t.Errorf("expected valid config with multi-type source to pass, got: %v", err)
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

func TestValidateConfig_WizardStepsValid(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Forms: map[string]Form{
			"wizard": {
				EntityType: "ticket",
				Steps: []FormStep{
					{Title: "Basics", Fields: []FormField{{Property: "title"}}},
					{
						Title:       "Details",
						VisibleWhen: "form.status == 'open'",
						Fields:      []FormField{{Property: "priority", RequiredWhen: "form.status == 'open'"}},
						Relations:   []FormRelation{{Relation: "blocks", Direction: "outgoing", Widget: "select"}},
					},
				},
			},
		},
	}

	if err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta); err != nil {
		t.Fatalf("expected valid wizard form, got: %v", err)
	}
}

func TestValidateConfig_WizardStepsAndFlatMutuallyExclusive(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Forms: map[string]Form{
			"mixed": {
				EntityType: "ticket",
				Fields:     []FormField{{Property: "title"}},
				Steps:      []FormStep{{Title: "Step 1", Fields: []FormField{{Property: "status"}}}},
			},
		},
	}

	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil {
		t.Fatal("expected error when both fields and steps are set")
	}
	if !strings.Contains(err.Error(), "either top-level fields/relations OR steps") {
		t.Errorf("expected steps-xor-flat error, got: %v", err)
	}
}

func TestValidateConfig_WizardStepUnknownProperty(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Forms: map[string]Form{
			"wizard": {
				EntityType: "ticket",
				Steps: []FormStep{
					{Title: "Step 1", Fields: []FormField{{Property: "unknown_prop"}}},
				},
			},
		},
	}

	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil {
		t.Fatal("expected error for unknown property in a step")
	}
	if !strings.Contains(err.Error(), `step[0] field[0] property "unknown_prop" not in metamodel`) {
		t.Errorf("expected step-scoped unknown-property error, got: %v", err)
	}
}

func TestValidateConfig_WizardStepMissingTitle(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Forms: map[string]Form{
			"wizard": {
				EntityType: "ticket",
				Steps:      []FormStep{{Fields: []FormField{{Property: "title"}}}},
			},
		},
	}

	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil {
		t.Fatal("expected error for step without title")
	}
	if !strings.Contains(err.Error(), "step[0] has no title") {
		t.Errorf("expected missing-title error, got: %v", err)
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
	// `=~` is regex syntax from rela's OTHER filter languages (search,
	// calfeed where, CLI) and was wrongly accepted here for months while
	// no layer below could evaluate it (the SPA degraded it to `eq`).
	cfg := &Config{
		Lists: map[string]List{
			"test": {
				EntityType: "ticket",
				Filters:    []FilterConfig{{Property: "status", Operator: "=~"}},
			},
		},
	}

	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil {
		t.Fatal("expected error for invalid filter operator")
	}
	if !strings.Contains(err.Error(), `invalid operator "=~"`) {
		t.Errorf("expected error about invalid operator, got: %v", err)
	}
}

// TestValidateConfig_DocumentedFilterOperatorsAccepted pins the full
// documented operator set (docs/data-entry.md "Static Filters") so the
// validator can't drift from the docs/SPA/API again — `in` and `~` were
// documented and evaluable but rejected here until this test existed.
func TestValidateConfig_DocumentedFilterOperatorsAccepted(t *testing.T) {
	meta := testMetamodel()
	for _, op := range []string{"=", "==", "!=", "~", "<", "<=", ">", ">=", "in"} {
		cfg := &Config{
			Lists: map[string]List{
				"test": {
					EntityType: "ticket",
					Filters:    []FilterConfig{{Property: "status", Operator: op, Value: "x"}},
				},
			},
		}
		if err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta); err != nil {
			t.Errorf("operator %q: expected valid, got: %v", op, err)
		}
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

func TestValidateConfig_ViewsDuplicateEntryType(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Views: map[string]ViewConfig{
			"ticket_a": {Entry: ViewEntry{Type: "ticket"}},
			"ticket_b": {Entry: ViewEntry{Type: "ticket"}},
		},
	}

	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil {
		t.Fatal("expected error for duplicate view entry types")
	}
	msg := err.Error()
	// Both view IDs must appear so the project owner can locate them.
	if !strings.Contains(msg, "ticket_a") || !strings.Contains(msg, "ticket_b") {
		t.Errorf("expected error to list both duplicate view IDs, got: %v", err)
	}
	if !strings.Contains(msg, `entity type "ticket"`) {
		t.Errorf("expected error to name the conflicting entity type, got: %v", err)
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

// TestValidateConfig_Documents is a table-driven sweep of DocumentConfig
// validation. Covers the {command, script} mutual-exclusion rule and the
// still-required entity_type invariant (addresses RR-1FA8W, the risk that
// relaxing command's required-ness would silently drop entity_type).
func TestValidateConfig_Documents(t *testing.T) {
	meta := testMetamodel()

	// A minimal valid form so doc.edit.form references resolve.
	editForm := Form{EntityType: "ticket", Title: "Edit Requirement", Mode: "edit"}

	cases := []struct {
		name    string
		doc     DocumentConfig
		wantErr string // substring that must appear; "" means expect success
	}{
		{
			name:    "only command is valid",
			doc:     DocumentConfig{Command: "render.sh", EntityType: "ticket"},
			wantErr: "",
		},
		{
			name:    "only script is valid",
			doc:     DocumentConfig{Script: "docs/render.lua", EntityType: "ticket"},
			wantErr: "",
		},
		{
			name:    "both command and script is an error",
			doc:     DocumentConfig{Command: "render.sh", Script: "docs/render.lua", EntityType: "ticket"},
			wantErr: "mutually exclusive",
		},
		{
			name:    "neither command nor script is an error",
			doc:     DocumentConfig{EntityType: "ticket"},
			wantErr: "one of command or script must be set",
		},
		{
			name:    "missing entity_type with command",
			doc:     DocumentConfig{Command: "render.sh"},
			wantErr: "entity_type is required",
		},
		{
			name:    "missing entity_type with script",
			doc:     DocumentConfig{Script: "docs/render.lua"},
			wantErr: "entity_type is required",
		},
		{
			name: "edit block with valid form and label",
			doc: DocumentConfig{
				Command:    "render.sh",
				EntityType: "ticket",
				Edit:       &DocumentEdit{Form: "edit_req", Label: "Edit requirement"},
			},
			wantErr: "",
		},
		{
			name: "edit.form references unknown form",
			doc: DocumentConfig{
				Command:    "render.sh",
				EntityType: "ticket",
				Edit:       &DocumentEdit{Form: "bogus", Label: "Edit"},
			},
			wantErr: `edit.form references unknown form "bogus"`,
		},
		{
			name: "edit.form empty when edit block is set",
			doc: DocumentConfig{
				Command:    "render.sh",
				EntityType: "ticket",
				Edit:       &DocumentEdit{Form: "", Label: "Edit"},
			},
			wantErr: "edit.form is required when edit is set",
		},
		{
			name: "edit.label empty when edit block is set",
			doc: DocumentConfig{
				Command:    "render.sh",
				EntityType: "ticket",
				Edit:       &DocumentEdit{Form: "edit_req", Label: ""},
			},
			wantErr: "edit.label is required when edit is set",
		},
		{
			name: "edit block with script renderer",
			doc: DocumentConfig{
				Script:     "docs/render.lua",
				EntityType: "ticket",
				Edit:       &DocumentEdit{Form: "edit_req", Label: "Edit"},
			},
			wantErr: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{
				Documents: map[string]DocumentConfig{"spec": tc.doc},
				Forms:     map[string]Form{"edit_req": editForm},
			}
			err := ValidateConfig(nil, cfg, meta)
			if tc.wantErr == "" {
				if err != nil {
					t.Errorf("expected success, got error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("expected error to contain %q, got: %s", tc.wantErr, err.Error())
			}
		})
	}
}

// TestValidateConfig_DocumentsEditBothEmpty pins the contract that an empty
// DocumentEdit produces both error messages. The independent if-branches in
// validateDocuments are correct only if both fire for `Edit: &DocumentEdit{}`.
func TestValidateConfig_DocumentsEditBothEmpty(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Documents: map[string]DocumentConfig{
			"spec": {
				Command:    "render.sh",
				EntityType: "ticket",
				Edit:       &DocumentEdit{}, // both fields empty
			},
		},
	}
	err := ValidateConfig(nil, cfg, meta)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "edit.form is required when edit is set") {
		t.Errorf("expected edit.form error, got: %s", msg)
	}
	if !strings.Contains(msg, "edit.label is required when edit is set") {
		t.Errorf("expected edit.label error, got: %s", msg)
	}
}

// TestValidateConfig_DocumentsEditFormSuggestion verifies the typo-suggestion
// path matches list.edit_form's user-facing wording.
func TestValidateConfig_DocumentsEditFormSuggestion(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Documents: map[string]DocumentConfig{
			"spec": {
				Command:    "render.sh",
				EntityType: "ticket",
				Edit:       &DocumentEdit{Form: "EDIT_TICKET", Label: "Edit"},
			},
		},
		Forms: map[string]Form{"edit_ticket": {EntityType: "ticket", Title: "Edit"}},
	}
	err := ValidateConfig(nil, cfg, meta)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `did you mean "edit_ticket"`) {
		t.Errorf("expected typo suggestion, got: %s", err.Error())
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

func TestValidateActions_ValidScript(t *testing.T) {
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

func TestValidateActions_ValidSet(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Actions: map[string]Action{
			"mark-done": {
				Label: "Done",
				Key:   "d",
				Set:   map[string]string{"status": "closed"},
			},
		},
		Lists: map[string]List{
			"tickets": {
				EntityType: "ticket",
				Columns:    []ListColumn{{Property: "title"}},
				Actions:    []string{"mark-done"},
			},
		},
	}
	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidAppID(t *testing.T) {
	valid := []string{"dash", "ticket-counter", "a", "a_b-c", strings.Repeat("a", 64)}
	for _, id := range valid {
		if !ValidAppID(id) {
			t.Errorf("ValidAppID(%q) = false, want true", id)
		}
	}
	invalid := []string{"", "Dash", "has space", "has.dot", "has/slash", "..", strings.Repeat("a", 65)}
	for _, id := range invalid {
		if ValidAppID(id) {
			t.Errorf("ValidAppID(%q) = true, want false", id)
		}
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

func TestValidateActions_NeitherSetNorScript(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Actions: map[string]Action{"foo": {}},
	}
	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil || !strings.Contains(err.Error(), "neither script nor set") {
		t.Errorf("expected neither error, got: %v", err)
	}
}

func TestValidateActions_BothSetAndScript(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Actions: map[string]Action{
			"foo": {Script: "x.lua", Set: map[string]string{"status": "closed"}},
		},
	}
	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil || !strings.Contains(err.Error(), "both script and set") {
		t.Errorf("expected both error, got: %v", err)
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

func TestValidateActions_ReservedKey(t *testing.T) {
	meta := testMetamodel()
	for _, key := range []string{"j", "k", "o", "e", "n", "h", "l"} {
		t.Run(key, func(t *testing.T) {
			cfg := &Config{
				Actions: map[string]Action{
					"act": {Label: "Act", Key: key, Set: map[string]string{"status": "closed"}},
				},
			}
			err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
			if err == nil || !strings.Contains(err.Error(), "reserved") {
				t.Errorf("expected reserved key error for %q, got: %v", key, err)
			}
		})
	}
}

func TestValidateActions_InvalidKeyFormat(t *testing.T) {
	meta := testMetamodel()
	for _, key := range []string{"dd", "D", "!", " ", "Enter"} {
		t.Run(key, func(t *testing.T) {
			cfg := &Config{
				Actions: map[string]Action{
					"act": {Label: "Act", Key: key, Set: map[string]string{"status": "closed"}},
				},
			}
			err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
			if err == nil || !strings.Contains(err.Error(), "single lowercase") {
				t.Errorf("expected key format error for %q, got: %v", key, err)
			}
		})
	}
}

func TestValidateActions_DuplicateKeysInList(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Actions: map[string]Action{
			"act1": {Label: "One", Key: "d", Set: map[string]string{"status": "closed"}},
			"act2": {Label: "Two", Key: "d", Set: map[string]string{"status": "open"}},
		},
		Lists: map[string]List{
			"tickets": {
				EntityType: "ticket",
				Columns:    []ListColumn{{Property: "title"}},
				Actions:    []string{"act1", "act2"},
			},
		},
	}
	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil || !strings.Contains(err.Error(), "duplicate key") {
		t.Errorf("expected duplicate key error, got: %v", err)
	}
}

func TestValidateActions_UnknownActionRef(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Lists: map[string]List{
			"tickets": {
				EntityType: "ticket",
				Columns:    []ListColumn{{Property: "title"}},
				Actions:    []string{"nonexistent"},
			},
		},
	}
	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil || !strings.Contains(err.Error(), "unknown action") {
		t.Errorf("expected unknown action error, got: %v", err)
	}
}

func TestValidateActions_MissingLabelInList(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Actions: map[string]Action{
			"act": {Key: "d", Set: map[string]string{"status": "closed"}},
		},
		Lists: map[string]List{
			"tickets": {
				EntityType: "ticket",
				Columns:    []ListColumn{{Property: "title"}},
				Actions:    []string{"act"},
			},
		},
	}
	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil || !strings.Contains(err.Error(), "must have a label") {
		t.Errorf("expected missing label error, got: %v", err)
	}
}

func TestValidateActions_MissingKeyInList(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Actions: map[string]Action{
			"act": {Label: "Done", Set: map[string]string{"status": "closed"}},
		},
		Lists: map[string]List{
			"tickets": {
				EntityType: "ticket",
				Columns:    []ListColumn{{Property: "title"}},
				Actions:    []string{"act"},
			},
		},
	}
	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil || !strings.Contains(err.Error(), "must have a key") {
		t.Errorf("expected missing key error, got: %v", err)
	}
}

func TestValidateActions_UnknownPropertyInSet(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Actions: map[string]Action{
			"act": {Label: "Done", Key: "d", Set: map[string]string{"nonexistent": "val"}},
		},
		Lists: map[string]List{
			"tickets": {
				EntityType: "ticket",
				Columns:    []ListColumn{{Property: "title"}},
				Actions:    []string{"act"},
			},
		},
	}
	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil || !strings.Contains(err.Error(), "unknown property") {
		t.Errorf("expected unknown property error, got: %v", err)
	}
}

func TestValidateActions_ScriptActionWithoutKeyLabel(t *testing.T) {
	// Script-only actions (sidebar) don't need key/label when not referenced by a list
	meta := testMetamodel()
	cfg := &Config{
		Actions: map[string]Action{
			"sidebar-action": {Script: "do-stuff.lua"},
		},
	}
	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
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

func TestValidateEntityViews_Valid(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Views: map[string]ViewConfig{
			"ticket_detail": {Entry: ViewEntry{Type: "ticket"}},
		},
		EntityViews: map[string]EntityViewConfig{
			"ticket": {DetailView: "ticket_detail"},
		},
	}
	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateEntityViews_AbsentIsValid(t *testing.T) {
	// The pre-migration baseline: no entity_views key at all must validate.
	meta := testMetamodel()
	cfg := &Config{}
	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateEntityViews_UnknownEntityType(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Views: map[string]ViewConfig{
			"any_view": {Entry: ViewEntry{Type: "ticket"}},
		},
		EntityViews: map[string]EntityViewConfig{
			"unknown_type": {DetailView: "any_view"},
		},
	}
	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil || !strings.Contains(err.Error(), `entity_views: unknown entity type "unknown_type"`) {
		t.Errorf("expected unknown entity type error, got: %v", err)
	}
}

func TestValidateEntityViews_EmptyDetailViewIsError(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		EntityViews: map[string]EntityViewConfig{
			"ticket": {DetailView: ""},
		},
	}
	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil || !strings.Contains(err.Error(), "detail_view is empty") {
		t.Errorf("expected empty detail_view error, got: %v", err)
	}
}

func TestValidateEntityViews_UnknownDetailView(t *testing.T) {
	meta := testMetamodel()
	cfg := &Config{
		Views: map[string]ViewConfig{
			"ticket_detail": {Entry: ViewEntry{Type: "ticket"}},
		},
		EntityViews: map[string]EntityViewConfig{
			"ticket": {DetailView: "nonexistent_view"},
		},
	}
	err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
	if err == nil || !strings.Contains(err.Error(), `references unknown view "nonexistent_view"`) {
		t.Errorf("expected unknown view error, got: %v", err)
	}
}

func TestValidateConfig_UnknownTopLevelKey_EntityViewsAccepted(t *testing.T) {
	// Regression: entity_views: must be in validTopLevelKeys.
	meta := testMetamodel()
	yamlData := []byte(`
version: "1.0"
entity_views:
  ticket:
    detail_view: ticket_detail
views:
  ticket_detail:
    entry:
      type: ticket
`)
	cfg := &Config{
		Views: map[string]ViewConfig{
			"ticket_detail": {Entry: ViewEntry{Type: "ticket"}},
		},
		EntityViews: map[string]EntityViewConfig{
			"ticket": {DetailView: "ticket_detail"},
		},
	}
	err := ValidateConfig(yamlData, cfg, meta)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateApp_PlantUMLServerURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr string // substring; "" means valid
	}{
		{name: "empty is valid (disabled)", url: "", wantErr: ""},
		{name: "https is valid", url: "https://plantuml.example.com", wantErr: ""},
		{name: "http is valid", url: "http://localhost:8080", wantErr: ""},
		{name: "https with path is valid", url: "https://example.com/plantuml", wantErr: ""},
		{name: "javascript scheme rejected", url: "javascript:alert(1)", wantErr: "scheme must be http or https"},
		{name: "data scheme rejected", url: "data:text/html,x", wantErr: "scheme must be http or https"},
		{name: "protocol-relative rejected", url: "//evil.example", wantErr: "scheme must be http or https"},
		{name: "bare host rejected", url: "evil.example/x", wantErr: "scheme must be http or https"},
		{name: "scheme without host rejected", url: "https://", wantErr: "must include a host"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{App: AppConfig{PlantUMLServerURL: tt.url}}
			errs := validateApp(cfg)
			if tt.wantErr == "" {
				if len(errs) != 0 {
					t.Fatalf("validateApp(%q) = %v, want no errors", tt.url, errs)
				}
				return
			}
			if len(errs) == 0 {
				t.Fatalf("validateApp(%q) = no errors, want error containing %q", tt.url, tt.wantErr)
			}
			if !strings.Contains(errs[0], tt.wantErr) {
				t.Errorf("validateApp(%q) = %q, want substring %q", tt.url, errs[0], tt.wantErr)
			}
		})
	}
}

func TestCollectConfigWarnings_IncomingRelationFilterWrongSide(t *testing.T) {
	meta := testMetamodel() // belongs-to: from ticket, to category

	tests := []struct {
		name        string
		list        List
		wantWarning bool
	}{
		{
			name: "incoming filter on the `to` side is fine",
			list: List{
				EntityType:     "category",
				FilterControls: []FilterControl{{Relation: "belongs-to", Direction: DirectionIncoming}},
			},
			wantWarning: false,
		},
		{
			name: "incoming filter on the `from` side warns",
			list: List{
				EntityType:     "ticket",
				FilterControls: []FilterControl{{Relation: "belongs-to", Direction: DirectionIncoming}},
			},
			wantWarning: true,
		},
		{
			name: "outgoing filter on the `from` side is fine (no direction check)",
			list: List{
				EntityType:     "ticket",
				FilterControls: []FilterControl{{Relation: "belongs-to"}},
			},
			wantWarning: false,
		},
		{
			name: "property filter control never warns",
			list: List{
				EntityType:     "ticket",
				FilterControls: []FilterControl{{Property: "status"}},
			},
			wantWarning: false,
		},
		{
			name: "incoming filter on a self-referential relation is fine",
			list: List{
				EntityType:     "ticket",
				FilterControls: []FilterControl{{Relation: "blocks", Direction: DirectionIncoming}},
			},
			wantWarning: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Lists: map[string]List{"lst": tt.list}}
			warnings := CollectConfigWarnings(cfg, meta)
			if tt.wantWarning {
				if len(warnings) == 0 {
					t.Fatalf("expected a warning, got none")
				}
				if !strings.Contains(warnings[0], "belongs-to") ||
					!strings.Contains(warnings[0], "incoming") {

					t.Errorf("warning missing expected detail: %q", warnings[0])
				}
			} else if len(warnings) != 0 {
				t.Errorf("expected no warnings, got %v", warnings)
			}
		})
	}
}

func TestCollectConfigWarnings_NotFatal(t *testing.T) {
	meta := testMetamodel()
	// A wrong-side incoming filter is a warning, NOT a hard validation error.
	cfg := &Config{
		Lists: map[string]List{
			"lst": {
				EntityType:     "ticket",
				FilterControls: []FilterControl{{Relation: "belongs-to", Direction: DirectionIncoming}},
			},
		},
	}
	if err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta); err != nil {
		t.Fatalf("wrong-side incoming filter should not be a fatal error, got: %v", err)
	}
	if len(CollectConfigWarnings(cfg, meta)) == 0 {
		t.Error("expected a non-fatal warning for the wrong-side incoming filter")
	}
}

// TestCollectConfigWarnings_ConflictingRelationDirections pins RR-9MJRJG: two
// lists of the same entity type configuring the same relation with conflicting
// directions must produce a warning naming the winning (lowest) list ID.
func TestCollectConfigWarnings_ConflictingRelationDirections(t *testing.T) {
	meta := testMetamodel()

	t.Run("conflict warns and names lowest list ID winner", func(t *testing.T) {
		cfg := &Config{Lists: map[string]List{
			"zzz_open": {
				EntityType:     "category",
				FilterControls: []FilterControl{{Relation: "belongs-to", Direction: DirectionIncoming}},
			},
			"aaa_open": {
				EntityType:     "category",
				FilterControls: []FilterControl{{Relation: "belongs-to", Direction: DirectionOutgoing}},
			},
		}}
		warnings := CollectConfigWarnings(cfg, meta)
		found := false
		for _, w := range warnings {
			if strings.Contains(w, "conflicting directions") && strings.Contains(w, `"aaa_open"`) {
				found = true
			}
		}
		if !found {
			t.Fatalf("expected a conflicting-directions warning naming aaa_open, got %v", warnings)
		}
	})

	t.Run("agreeing directions across lists do not warn", func(t *testing.T) {
		cfg := &Config{Lists: map[string]List{
			"a": {EntityType: "ticket", FilterControls: []FilterControl{{Relation: "belongs-to"}}},
			"b": {EntityType: "ticket", FilterControls: []FilterControl{{Relation: "belongs-to", Direction: DirectionOutgoing}}},
		}}
		for _, w := range CollectConfigWarnings(cfg, meta) {
			if strings.Contains(w, "conflicting directions") {
				t.Errorf("unexpected conflict warning: %q", w)
			}
		}
	})
}

// TestCollectConfigWarnings_RelationPropertyNameCollision pins RR-0HWAS0: a
// relation FilterControl whose name also names a property of the list's entity
// type (disjoint namespaces) is flagged, because the runtime routes the filter
// to the property.
func TestCollectConfigWarnings_RelationPropertyNameCollision(t *testing.T) {
	// taak has property "belongs_to"; the metamodel also has a global relation
	// "belongs_to". Configuring a relation filter on it collides.
	meta := &metamodel.Metamodel{
		Version: "1.0",
		Entities: map[string]metamodel.EntityDef{
			"taak": {
				Label:    "Taak",
				IDPrefix: "TAAK-",
				Properties: map[string]metamodel.PropertyDef{
					"title":      {Type: "string", Required: true},
					"belongs_to": {Type: "string"},
				},
			},
			"project": {Label: "Project", IDPrefix: "PRJ-", Properties: map[string]metamodel.PropertyDef{"name": {Type: "string"}}},
		},
		Relations: map[string]metamodel.RelationDef{
			"belongs_to": {Label: "belongs to", From: []string{"taak"}, To: []string{"project"}},
		},
	}

	t.Run("relation control colliding with a property warns", func(t *testing.T) {
		cfg := &Config{Lists: map[string]List{
			"taken": {EntityType: "taak", FilterControls: []FilterControl{{Relation: "belongs_to"}}},
		}}
		warnings := CollectConfigWarnings(cfg, meta)
		found := false
		for _, w := range warnings {
			if strings.Contains(w, "collides with a property") && strings.Contains(w, "belongs_to") {
				found = true
			}
		}
		if !found {
			t.Fatalf("expected a name-collision warning, got %v", warnings)
		}
	})

	t.Run("explicit property control disambiguates, no warning", func(t *testing.T) {
		cfg := &Config{Lists: map[string]List{
			"taken": {EntityType: "taak", FilterControls: []FilterControl{
				{Property: "belongs_to"},
				{Relation: "belongs_to"},
			}},
		}}
		for _, w := range CollectConfigWarnings(cfg, meta) {
			if strings.Contains(w, "collides with a property") {
				t.Errorf("unexpected collision warning when a property control disambiguates: %q", w)
			}
		}
	})
}

// TestViewCommandPermissionWarning pins that a `permission:` on a view command
// is surfaced as a warning. The key is inert there (TKT-MJ02AO), and silently
// ignoring it would let an author believe they had granted access.
func TestViewCommandPermissionWarning(t *testing.T) {
	meta := &metamodel.Metamodel{}

	t.Run("view command with permission warns", func(t *testing.T) {
		cfg := &Config{Commands: map[string]CommandConfig{
			"v": {Label: "V", Script: "echo hi", Context: "view", Permission: "command:v"},
		}}
		warnings := CollectConfigWarnings(cfg, meta)
		var found bool
		for _, w := range warnings {
			if strings.Contains(w, `command "v"`) && strings.Contains(w, "ignored for context: view") {
				found = true
			}
		}
		if !found {
			t.Errorf("expected a view-permission warning, got %v", warnings)
		}
	})

	t.Run("view command without permission is silent", func(t *testing.T) {
		cfg := &Config{Commands: map[string]CommandConfig{
			"v": {Label: "V", Script: "echo hi", Context: "view"},
		}}
		for _, w := range CollectConfigWarnings(cfg, meta) {
			if strings.Contains(w, "ignored for context: view") {
				t.Errorf("unexpected warning: %s", w)
			}
		}
	})

	t.Run("non-view command with permission is silent", func(t *testing.T) {
		cfg := &Config{Commands: map[string]CommandConfig{
			"g": {Label: "G", Script: "echo hi", Context: "global", Permission: "command:g"},
		}}
		for _, w := range CollectConfigWarnings(cfg, meta) {
			if strings.Contains(w, "ignored for context: view") {
				t.Errorf("unexpected warning: %s", w)
			}
		}
	})
}
