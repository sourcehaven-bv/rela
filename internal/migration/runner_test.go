package migration

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// A metamodel passed to one run must not reach a later run without one. The
// registered migrations are process-wide singletons, so binding the metamodel
// by mutating them leaked it across runs (and raced between concurrent ones).
func TestDetectFromNodeWithMetamodel_DoesNotLeakMetamodelAcrossRuns(t *testing.T) {
	const input = "forms:\n  ticket:\n    entity_type: ticket\n    fields:\n      - property: title\n        widget: text\n"
	detect := func(meta MetamodelProvider) []DetectionResult {
		t.Helper()
		var doc yaml.Node
		if err := yaml.Unmarshal([]byte(input), &doc); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		return DetectFromNodeWithMetamodel(&doc, FileTypeDataEntry, meta)
	}

	detect(&mockMetamodel{})

	for _, d := range detect(nil) {
		if strings.Contains(d.Description, "using metamodel") {
			t.Errorf("run without a metamodel reported %q", d.Description)
		}
	}
}
