package dataentry

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// The schema's label_plural reaches the SPA trimmed, and is absent when
// the schema sets none or only blanks, so the SPA never shows a guessed plural such as
// "Persons" (BUG-ST02A0S).
func TestToV1EntityType_LabelPlural(t *testing.T) {
	meta := &metamodel.Metamodel{}

	set := toV1EntityType(meta, "person", metamodel.EntityDef{Label: "Person", LabelPlural: "People"})
	if set.LabelPlural != "People" {
		t.Errorf("LabelPlural = %q, want People", set.LabelPlural)
	}

	blank := toV1EntityType(meta, "person", metamodel.EntityDef{Label: "Person", LabelPlural: "  "})
	if blank.LabelPlural != "" {
		t.Errorf("LabelPlural = %q, want empty for a blank label", blank.LabelPlural)
	}

	unset := toV1EntityType(meta, "person", metamodel.EntityDef{Label: "Person", Plural: "people"})
	if unset.LabelPlural != "" {
		t.Errorf("LabelPlural = %q, want empty", unset.LabelPlural)
	}
	raw, err := json.Marshal(unset)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "label_plural") {
		t.Errorf("wire carries label_plural without a schema value: %s", raw)
	}
}
