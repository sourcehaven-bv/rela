package migration

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestViewsByEntityType_Detect(t *testing.T) {
	m := &ViewsByEntityTypeMigration{}

	tests := []struct {
		name   string
		yaml   string
		expect bool
	}{
		{
			name: "detects view keyed by view-id rather than entity-type",
			yaml: `
views:
  ticket_detail:
    entry:
      type: ticket
`,
			expect: true,
		},
		{
			name: "no detection when key already matches entity type",
			yaml: `
views:
  ticket:
    entry:
      type: ticket
`,
			expect: false,
		},
		{
			name: "detects detail_view in list config",
			yaml: `
lists:
  all_tickets:
    detail_view: ticket_detail
`,
			expect: true,
		},
		{
			name: "no detection when no views and no detail_view",
			yaml: `
forms:
  create_ticket:
    entity_type: ticket
`,
			expect: false,
		},
		{
			name: "view without entry.type is not detected — validator's job",
			yaml: `
views:
  malformed:
    title: "Missing entry"
`,
			expect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var doc yaml.Node
			if err := yaml.Unmarshal([]byte(tt.yaml), &doc); err != nil {
				t.Fatalf("parse YAML: %v", err)
			}
			if got := m.Detect(&doc); got != tt.expect {
				t.Errorf("Detect() = %v, want %v", got, tt.expect)
			}
		})
	}
}

func TestViewsByEntityType_Apply_RekeysViews(t *testing.T) {
	m := &ViewsByEntityTypeMigration{}

	src := `
views:
  ticket_detail:
    title: "Ticket"
    entry:
      type: ticket
    sections:
      - heading: "Ticket"
        source: entry
        display: properties
  feature_detail:
    title: "Feature"
    entry:
      type: feature
`

	doc := mustParseYAML(t, src)
	if err := m.Apply(&doc); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	out := mustMarshalYAML(t, &doc)
	// New keys should be present, old keys gone.
	if !strings.Contains(out, "  ticket:") {
		t.Errorf("expected re-keyed `ticket:` in output, got:\n%s", out)
	}
	if !strings.Contains(out, "  feature:") {
		t.Errorf("expected re-keyed `feature:` in output, got:\n%s", out)
	}
	if strings.Contains(out, "ticket_detail:") || strings.Contains(out, "feature_detail:") {
		t.Errorf("old view-id keys should be gone, got:\n%s", out)
	}
	// Inner structure must be preserved.
	if !strings.Contains(out, "Ticket") || !strings.Contains(out, "Feature") {
		t.Errorf("titles should be preserved, got:\n%s", out)
	}
}

func TestViewsByEntityType_Apply_StripsEntityViewsBlock(t *testing.T) {
	m := &ViewsByEntityTypeMigration{}

	src := `
entity_views:
  ticket:
    detail_view: ticket_detail
forms:
  create_ticket:
    entity_type: ticket
`
	doc := mustParseYAML(t, src)
	if err := m.Apply(&doc); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	out := mustMarshalYAML(t, &doc)
	if strings.Contains(out, "entity_views") {
		t.Errorf("entity_views should be removed entirely, got:\n%s", out)
	}
	if !strings.Contains(out, "forms:") {
		t.Errorf("unrelated config should be preserved, got:\n%s", out)
	}
}

func TestViewsByEntityType_Detect_OnEntityViewsAlone(t *testing.T) {
	m := &ViewsByEntityTypeMigration{}

	src := `
entity_views:
  ticket:
    detail_view: ticket_detail
`
	doc := mustParseYAML(t, src)
	if !m.Detect(&doc) {
		t.Error("expected Detect=true when only entity_views: is present")
	}
}

func TestViewsByEntityType_Apply_StripsDetailViewFromLists(t *testing.T) {
	m := &ViewsByEntityTypeMigration{}

	src := `
lists:
  all_tickets:
    entity: ticket
    detail_view: ticket_detail
    columns:
      - property: title
  all_features:
    entity: feature
    columns:
      - property: title
`

	doc := mustParseYAML(t, src)
	if err := m.Apply(&doc); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	out := mustMarshalYAML(t, &doc)
	if strings.Contains(out, "detail_view") {
		t.Errorf("detail_view should be removed, got:\n%s", out)
	}
	// Untouched fields must remain.
	if !strings.Contains(out, "all_tickets:") || !strings.Contains(out, "entity: ticket") {
		t.Errorf("list config should be preserved otherwise, got:\n%s", out)
	}
}

func TestViewsByEntityType_Apply_ErrorsOnDuplicateEntityType(t *testing.T) {
	m := &ViewsByEntityTypeMigration{}

	src := `
views:
  ticket_detail_a:
    entry:
      type: ticket
  ticket_detail_b:
    entry:
      type: ticket
`
	doc := mustParseYAML(t, src)
	err := m.Apply(&doc)
	if err == nil {
		t.Fatal("expected error for duplicate entity types")
	}
	msg := err.Error()
	if !strings.Contains(msg, "ticket_detail_a") || !strings.Contains(msg, "ticket_detail_b") {
		t.Errorf("error should name both view IDs, got: %v", err)
	}
	if !strings.Contains(msg, `entity type "ticket"`) {
		t.Errorf("error should name conflicting entity type, got: %v", err)
	}
}

func TestViewsByEntityType_Apply_Idempotent(t *testing.T) {
	m := &ViewsByEntityTypeMigration{}

	// Already-migrated input — keys match entry.type, no detail_view.
	src := `
views:
  ticket:
    entry:
      type: ticket
lists:
  all_tickets:
    entity: ticket
`

	doc := mustParseYAML(t, src)
	before := mustMarshalYAML(t, &doc)

	if err := m.Apply(&doc); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	after := mustMarshalYAML(t, &doc)
	if before != after {
		t.Errorf("Apply should be a no-op on already-migrated YAML.\nBefore:\n%s\nAfter:\n%s", before, after)
	}
}

func TestViewsByEntityType_Apply_PreservesViewsWithoutEntryType(t *testing.T) {
	// A view missing entry.type can't be re-keyed; the migration should
	// skip it and let the validator complain at config-load time. The
	// test ensures we don't lose the view entirely.
	m := &ViewsByEntityTypeMigration{}

	src := `
views:
  malformed:
    title: "I have no entry.type"
  feature_detail:
    entry:
      type: feature
`
	doc := mustParseYAML(t, src)
	if err := m.Apply(&doc); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	out := mustMarshalYAML(t, &doc)
	if !strings.Contains(out, "malformed:") {
		t.Errorf("malformed view should be preserved, got:\n%s", out)
	}
	if !strings.Contains(out, "  feature:") {
		t.Errorf("feature_detail should be re-keyed to `feature:`, got:\n%s", out)
	}
}

func TestViewsByEntityType_Detect_OnEmptyDoc(t *testing.T) {
	m := &ViewsByEntityTypeMigration{}
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(``), &doc); err != nil {
		t.Fatalf("parse YAML: %v", err)
	}
	if m.Detect(&doc) {
		t.Error("expected Detect to be false on empty doc")
	}
}

func mustParseYAML(t *testing.T, s string) yaml.Node {
	t.Helper()
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(s), &doc); err != nil {
		t.Fatalf("parse YAML: %v", err)
	}
	return doc
}

func mustMarshalYAML(t *testing.T, doc *yaml.Node) string {
	t.Helper()
	out, err := yaml.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal YAML: %v", err)
	}
	return string(out)
}
