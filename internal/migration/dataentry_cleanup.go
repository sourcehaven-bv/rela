package migration

import (
	"slices"

	"gopkg.in/yaml.v3"
)

func init() {
	Register(&DataEntryCleanupMigration{})
}

// DataEntryCleanupMigration removes redundant properties from data-entry.yaml
// that can be auto-resolved at runtime. When provided with a metamodel (via
// SetMetamodel), it can detect and remove:
//   - widget matching the type→widget mapping
//   - required matching metamodel property
//   - default matching metamodel/type default
//   - target_type when single target in metamodel
//   - relation label matching the metamodel relation label
//
// Without a metamodel, it only removes widget: select.
//
// Every removal above is metamodel-grounded: the server re-derives the value
// from schema it owns, so the contract is verifiable. It deliberately does NOT
// remove a field or column label matching titleCase(property) — that was a
// convention-grounded removal depending on a client re-implementing an English
// title-casing transform, which silently downgraded labels to raw identifiers.
// See DEC-6C1NAA: a label is authored, never derived.
//
// It likewise does NOT remove `direction`, even when the form's entity type sits
// on only one side of the relation. That removal looked metamodel-grounded but
// failed the re-derivation test: nothing infers direction back. An absent
// `direction` is parsed as `outgoing` (Direction.UnmarshalYAML maps "" and
// "outgoing" to the same value) and the SPA widgets test `direction === 'incoming'`
// literally. So stripping `direction: incoming` from a to-side binding both flips
// the widget to the wrong side and makes ValidateConfig reject the file the
// migration just wrote — leaving a project that no longer starts.
type DataEntryCleanupMigration struct {
	meta MetamodelProvider
}

// SetMetamodel implements MetamodelAware.
func (m *DataEntryCleanupMigration) SetMetamodel(meta MetamodelProvider) {
	m.meta = meta
}

func (m *DataEntryCleanupMigration) Name() string {
	return "dataentry-cleanup"
}

func (m *DataEntryCleanupMigration) Description() string {
	if m.meta != nil {
		return "Remove redundant properties from data-entry.yaml (using metamodel)"
	}
	return "Remove default widgets from data-entry.yaml"
}

func (m *DataEntryCleanupMigration) FileTypes() []FileType {
	return []FileType{FileTypeDataEntry}
}

func (m *DataEntryCleanupMigration) Detect(doc *yaml.Node) bool {
	root := GetDocumentRoot(doc)
	if root == nil {
		return false
	}

	// Check forms section
	forms := GetMapValue(root, "forms")
	if forms != nil && forms.Kind == yaml.MappingNode {
		if m.detectInForms(forms) {
			return true
		}
	}

	// Lists are deliberately not inspected: a list column carries only a
	// label, and labels are never stripped (DEC-6C1NAA).
	return false
}

func (m *DataEntryCleanupMigration) detectInForms(forms *yaml.Node) bool {
	for i := 1; i < len(forms.Content); i += 2 {
		formDef := forms.Content[i]
		if formDef.Kind != yaml.MappingNode {
			continue
		}

		entityType := getScalarValue(formDef, "entity_type")
		if m.detectInFormFields(formDef, entityType) || m.detectInFormRelations(formDef, entityType) {
			return true
		}
	}
	return false
}

func (m *DataEntryCleanupMigration) detectInFormFields(formDef *yaml.Node, entityType string) bool {
	fields := GetMapValue(formDef, "fields")
	if fields == nil || fields.Kind != yaml.SequenceNode {
		return false
	}
	for _, field := range fields.Content {
		if field.Kind == yaml.MappingNode && m.isRedundantField(field, entityType) {
			return true
		}
	}
	return false
}

func (m *DataEntryCleanupMigration) detectInFormRelations(formDef *yaml.Node, entityType string) bool {
	relations := GetMapValue(formDef, "relations")
	if relations == nil || relations.Kind != yaml.SequenceNode {
		return false
	}
	for _, rel := range relations.Content {
		if rel.Kind == yaml.MappingNode && m.isRedundantRelation(rel, entityType) {
			return true
		}
	}
	return false
}

// isRedundantField checks if any property in this field is redundant.
//
// Note there is deliberately no label check here: a label is authored, never
// derived (DEC-6C1NAA). Stripping a label the consumer cannot reproduce
// silently downgrades it to a raw identifier, and because the server refuses
// to start on unmigrated config the user cannot decline the downgrade.
func (m *DataEntryCleanupMigration) isRedundantField(node *yaml.Node, entityType string) bool {
	return m.isRedundantWidget(node, entityType) ||
		m.isRedundantRequired(node, entityType) ||
		m.isRedundantDefault(node, entityType)
}

// isRedundantRelation checks if any property in this relation is redundant.
func (m *DataEntryCleanupMigration) isRedundantRelation(node *yaml.Node, entityType string) bool {
	return m.isRedundantRelationWidget(node) ||
		m.isRedundantRelationLabel(node) ||
		m.isRedundantTargetType(node, entityType)
}

// isRedundantWidget checks if widget matches the type→widget mapping.
func (m *DataEntryCleanupMigration) isRedundantWidget(node *yaml.Node, entityType string) bool {
	if m.meta == nil {
		return false
	}

	prop := getScalarValue(node, "property")
	widget := getScalarValue(node, "widget")
	if prop == "" || widget == "" {
		return false
	}

	propType := m.meta.GetPropertyType(entityType, prop)
	if propType == "" {
		return false
	}

	expectedWidget := m.meta.ResolveWidgetFromType(propType)
	return widget == expectedWidget
}

// isRedundantRequired checks if required matches metamodel.
func (m *DataEntryCleanupMigration) isRedundantRequired(node *yaml.Node, entityType string) bool {
	if m.meta == nil {
		return false
	}

	prop := getScalarValue(node, "property")
	requiredVal := getScalarValue(node, "required")
	if prop == "" || requiredVal == "" {
		return false
	}

	// Check if the required value matches metamodel
	metaRequired := m.meta.IsPropertyRequired(entityType, prop)
	formRequired := requiredVal == "true"
	return formRequired == metaRequired
}

// isRedundantDefault checks if default matches metamodel/type default.
func (m *DataEntryCleanupMigration) isRedundantDefault(node *yaml.Node, entityType string) bool {
	if m.meta == nil {
		return false
	}

	prop := getScalarValue(node, "property")
	defaultVal := getScalarValue(node, "default")
	if prop == "" || defaultVal == "" {
		return false
	}

	// Check property default first
	propDefault := m.meta.GetPropertyDefault(entityType, prop)
	if propDefault != "" {
		return defaultVal == propDefault
	}

	// Check custom type default
	propType := m.meta.GetPropertyType(entityType, prop)
	if propType != "" {
		typeDefault := m.meta.GetTypeDefault(propType)
		if typeDefault != "" {
			return defaultVal == typeDefault
		}
	}

	return false
}

// isRedundantRelationWidget checks if widget is "select" (the default).
func (m *DataEntryCleanupMigration) isRedundantRelationWidget(node *yaml.Node) bool {
	widget := getScalarValue(node, "widget")
	return widget == "select"
}

// isRedundantRelationLabel checks if a relation label duplicates the
// metamodel's own label for that relation type.
//
// This is the one label the migration may remove, because it is genuinely
// re-derivable: the metamodel label is server-authored, already served to the
// SPA via the schema API, and language-neutral — the SPA recovers it from
// `relationType.label`. That is a derivation from an AUTHORED label, not from
// an identifier, which is what DEC-6C1NAA forbids. The former titleCase(rel)
// arm was exactly such a forbidden derivation and has been removed.
func (m *DataEntryCleanupMigration) isRedundantRelationLabel(node *yaml.Node) bool {
	rel := getScalarValue(node, "relation")
	label := getScalarValue(node, "label")
	if rel == "" || label == "" || m.meta == nil {
		return false
	}

	relLabel := m.meta.GetRelationLabel(rel)
	return relLabel != "" && label == relLabel
}

// isRedundantTargetType checks if target_type can be inferred from metamodel.
func (m *DataEntryCleanupMigration) isRedundantTargetType(node *yaml.Node, entityType string) bool {
	if m.meta == nil {
		return false
	}

	rel := getScalarValue(node, "relation")
	targetType := getScalarValue(node, "target_type")
	direction := getScalarValue(node, "direction")
	if rel == "" || targetType == "" {
		return false
	}

	fromTypes := m.meta.GetRelationFrom(rel)
	toTypes := m.meta.GetRelationTo(rel)
	if len(fromTypes) == 0 && len(toTypes) == 0 {
		return false
	}

	// Infer direction if not specified
	if direction == "" && entityType != "" {
		inFrom := containsStr(fromTypes, entityType)
		inTo := containsStr(toTypes, entityType)
		if inFrom && !inTo {
			direction = "outgoing"
		} else if inTo && !inFrom {
			direction = "incoming"
		}
	}

	// Check if target_type matches the only possible target
	if direction == "incoming" && len(fromTypes) == 1 {
		return targetType == fromTypes[0]
	}
	if direction == "outgoing" && len(toTypes) == 1 {
		return targetType == toTypes[0]
	}

	return false
}

func (m *DataEntryCleanupMigration) Apply(doc *yaml.Node) error {
	root := GetDocumentRoot(doc)
	if root == nil {
		return nil
	}

	forms := GetMapValue(root, "forms")
	if forms != nil && forms.Kind == yaml.MappingNode {
		m.cleanupForms(forms)
	}

	// Lists are left untouched: a list column carries only a label, and
	// labels are never stripped (DEC-6C1NAA).
	return nil
}

func (m *DataEntryCleanupMigration) cleanupForms(forms *yaml.Node) {
	for i := 1; i < len(forms.Content); i += 2 {
		formDef := forms.Content[i]
		if formDef.Kind != yaml.MappingNode {
			continue
		}
		entityType := getScalarValue(formDef, "entity_type")
		m.cleanupFormFields(formDef, entityType)
		m.cleanupFormRelations(formDef, entityType)
	}
}

func (m *DataEntryCleanupMigration) cleanupFormFields(formDef *yaml.Node, entityType string) {
	fields := GetMapValue(formDef, "fields")
	if fields == nil || fields.Kind != yaml.SequenceNode {
		return
	}
	for _, field := range fields.Content {
		if field.Kind != yaml.MappingNode {
			continue
		}
		// No label removal here — see isRedundantField (DEC-6C1NAA).
		if m.isRedundantWidget(field, entityType) {
			DeleteMapKey(field, "widget")
		}
		if m.isRedundantRequired(field, entityType) {
			DeleteMapKey(field, "required")
		}
		if m.isRedundantDefault(field, entityType) {
			DeleteMapKey(field, "default")
		}
	}
}

func (m *DataEntryCleanupMigration) cleanupFormRelations(formDef *yaml.Node, entityType string) {
	relations := GetMapValue(formDef, "relations")
	if relations == nil || relations.Kind != yaml.SequenceNode {
		return
	}
	for _, rel := range relations.Content {
		if rel.Kind != yaml.MappingNode {
			continue
		}
		if m.isRedundantRelationWidget(rel) {
			DeleteMapKey(rel, "widget")
		}
		if m.isRedundantRelationLabel(rel) {
			DeleteMapKey(rel, "label")
		}
		if m.isRedundantTargetType(rel, entityType) {
			DeleteMapKey(rel, "target_type")
		}
	}
}

// Helper functions

func getScalarValue(node *yaml.Node, key string) string {
	val := GetMapValue(node, key)
	if val != nil && val.Kind == yaml.ScalarNode {
		return val.Value
	}
	return ""
}

func containsStr(slice []string, s string) bool {
	return slices.Contains(slice, s)
}
