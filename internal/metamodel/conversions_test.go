package metamodel

import (
	"strings"
	"testing"
)

const conversionSchema = `version: "1"
entities:
  note:
    label: Note
    id_prefix: NOTE-
    properties:
      title: {type: string}
  task:
    label: Task
    id_prefix: TASK-
    properties:
      title: {type: string}
relations: {}
conversions:
  note-to-task:
    from: note
    to: task
    fields:
      title: "{{source.title}}"
`

func TestConversionDefinitionsAreValidated(t *testing.T) {
	if _, err := Parse([]byte(conversionSchema)); err != nil {
		t.Fatalf("valid conversion rejected: %v", err)
	}
	if strings.Contains(conversionSchema, "faces:") {
		t.Fatal("faceless conversion example should not need a face mapping")
	}
	legacyCopySyntax := strings.Replace(conversionSchema, "{{source.title}}", "{{new.title}}", 1)
	if _, err := Parse([]byte(legacyCopySyntax)); err != nil {
		t.Fatalf("copy-compatible new.* reference rejected: %v", err)
	}
	bad := strings.Replace(conversionSchema, "to: task", "to: unknown", 1)
	if _, err := Parse([]byte(bad)); err == nil || !strings.Contains(err.Error(), "conversion") {
		t.Fatalf("unknown target error=%v", err)
	}
	bad = strings.Replace(conversionSchema, "title: \"{{source.title}}\"", "missing: \"{{source.title}}\"", 1)
	if _, err := Parse([]byte(bad)); err == nil || !strings.Contains(err.Error(), "property \"missing\"") {
		t.Fatalf("unknown destination property error=%v", err)
	}
	withRelation := strings.Replace(conversionSchema, "relations: {}", "relations:\n  related:\n    label: Related\n    from: [note]\n    to: [task]", 1)
	if _, err := Parse([]byte(withRelation)); err == nil || !strings.Contains(err.Error(), "needs an explicit mapping") {
		t.Fatalf("unmapped source relation error=%v", err)
	}
}

func TestConversionsReserveTheDropRelationName(t *testing.T) {
	withDrop := strings.Replace(conversionSchema, "relations: {}",
		"relations:\n  drop:\n    label: Drop\n    from: [task]\n    to: [task]", 1)
	if _, err := Parse([]byte(withDrop)); err == nil || !strings.Contains(err.Error(), `"drop" is reserved`) {
		t.Fatalf("relation named drop error=%v", err)
	}
	noConversions, _, _ := strings.Cut(withDrop, "conversions:")
	if _, err := Parse([]byte(noConversions)); err != nil {
		t.Fatalf("relation named drop rejected without conversions: %v", err)
	}
}
