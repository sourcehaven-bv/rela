package migration

import "gopkg.in/yaml.v3"

func init() {
	Register(&AutomationsToFlowsMigration{})
}

// AutomationsToFlowsMigration detects a schema that still declares
// `automations:` (TKT-GWOBH2U). Automations moved to flows.yaml under
// `triggers:`, and the runtime that read them from the schema is gone, so
// the detection is blocking: loading the schema would silently drop them.
//
// The move spans two files (the schema and flows.yaml) and the schema's
// includes, so it cannot be a per-file node rewrite. It is a [ProjectLevelMigration]:
// the per-file runner skips it and projectsetup performs the move.
type AutomationsToFlowsMigration struct{}

// AutomationsToFlowsName names the migration; the metamodel loader checks
// include files for it.
const AutomationsToFlowsName = "automations-to-flows"

// Name returns the migration identifier.
func (m *AutomationsToFlowsMigration) Name() string { return AutomationsToFlowsName }

// Description is the user-facing load error.
func (m *AutomationsToFlowsMigration) Description() string {
	return "automations: are declared in flows.yaml under triggers: since this release (TKT-GWOBH2U); " +
		"run 'rela migrate' to move them"
}

// FileTypes returns the schema file type.
func (m *AutomationsToFlowsMigration) FileTypes() []FileType {
	return []FileType{FileTypeMetamodel}
}

// ProjectLevel marks the migration as applied by projectsetup, not by
// [ApplyWithMetamodel].
func (m *AutomationsToFlowsMigration) ProjectLevel() bool { return true }

// Detect reports a top-level `automations:` key with any value other than
// null or an empty sequence. A value that is not a sequence (a mapping, a
// scalar) is detected too. The old loader would have refused it, and the
// move refuses it with a message, so it is never dropped without a word.
func (m *AutomationsToFlowsMigration) Detect(doc *yaml.Node) bool {
	root := GetDocumentRoot(doc)
	if root == nil || root.Kind != yaml.MappingNode {
		return false
	}
	return hasAutomations(GetMapValue(root, "automations"))
}

// hasAutomations reports whether v, the value of an `automations:` key,
// still needs the move. Nil, a YAML null and an empty sequence do not.
func hasAutomations(v *yaml.Node) bool {
	switch {
	case v == nil:
		return false
	case v.Kind == yaml.ScalarNode && v.Tag == "!!null":
		return false
	case v.Kind == yaml.SequenceNode && len(v.Content) == 0:
		return false
	}
	return true
}

// Apply is a no-op: the per-file runner never calls it (see [ProjectLevelMigration]).
func (m *AutomationsToFlowsMigration) Apply(_ *yaml.Node) error { return nil }
