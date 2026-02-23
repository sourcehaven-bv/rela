package migration

import (
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

func init() {
	Register(&DataEntryCleanupMigration{})
}

// DataEntryCleanupMigration removes redundant properties from data-entry.yaml
// that can be auto-resolved at runtime. When provided with a metamodel (via
// SetMetamodel), it can detect and remove:
//   - label matching titleCase(property)
//   - widget matching the type→widget mapping
//   - required matching metamodel property
//   - default matching metamodel/type default
//   - direction when unambiguous from metamodel
//   - target_type when single target in metamodel
//   - relation label matching metamodel relation label or titleCase
//
// Without a metamodel, it only removes labels matching titleCase and widget: select.
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
	return "Remove redundant labels and default widgets from data-entry.yaml"
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

	// Check lists section
	lists := GetMapValue(root, "lists")
	if lists != nil && lists.Kind == yaml.MappingNode {
		if m.detectInLists(lists) {
			return true
		}
	}

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

func (m *DataEntryCleanupMigration) detectInLists(lists *yaml.Node) bool {
	for i := 1; i < len(lists.Content); i += 2 {
		listDef := lists.Content[i]
		if listDef.Kind != yaml.MappingNode {
			continue
		}

		columns := GetMapValue(listDef, "columns")
		if columns != nil && columns.Kind == yaml.SequenceNode {
			for _, col := range columns.Content {
				if col.Kind == yaml.MappingNode && m.isRedundantLabel(col) {
					return true
				}
			}
		}
	}
	return false
}

// isRedundantField checks if any property in this field is redundant.
func (m *DataEntryCleanupMigration) isRedundantField(node *yaml.Node, entityType string) bool {
	return m.isRedundantLabel(node) ||
		m.isRedundantWidget(node, entityType) ||
		m.isRedundantRequired(node, entityType) ||
		m.isRedundantDefault(node, entityType)
}

// isRedundantRelation checks if any property in this relation is redundant.
func (m *DataEntryCleanupMigration) isRedundantRelation(node *yaml.Node, entityType string) bool {
	return m.isRedundantRelationWidget(node) ||
		m.isRedundantRelationLabel(node) ||
		m.isRedundantDirection(node, entityType) ||
		m.isRedundantTargetType(node, entityType)
}

// isRedundantLabel checks if label matches titleCase(property).
func (m *DataEntryCleanupMigration) isRedundantLabel(node *yaml.Node) bool {
	prop := getScalarValue(node, "property")
	label := getScalarValue(node, "label")
	if prop == "" || label == "" {
		return false
	}
	return label == titleCase(prop)
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

// isRedundantRelationLabel checks if relation label matches metamodel or titleCase.
func (m *DataEntryCleanupMigration) isRedundantRelationLabel(node *yaml.Node) bool {
	rel := getScalarValue(node, "relation")
	label := getScalarValue(node, "label")
	if rel == "" || label == "" {
		return false
	}

	// Check if matches metamodel label
	if m.meta != nil {
		relLabel := m.meta.GetRelationLabel(rel)
		if relLabel != "" && label == relLabel {
			return true
		}
	}

	// Check if matches titleCase
	return label == titleCase(rel)
}

// isRedundantDirection checks if direction can be inferred from metamodel.
func (m *DataEntryCleanupMigration) isRedundantDirection(node *yaml.Node, entityType string) bool {
	if m.meta == nil || entityType == "" {
		return false
	}

	rel := getScalarValue(node, "relation")
	direction := getScalarValue(node, "direction")
	if rel == "" || direction == "" {
		return false
	}

	fromTypes := m.meta.GetRelationFrom(rel)
	toTypes := m.meta.GetRelationTo(rel)
	if len(fromTypes) == 0 && len(toTypes) == 0 {
		return false
	}

	inFrom := containsStr(fromTypes, entityType)
	inTo := containsStr(toTypes, entityType)

	// Direction is redundant if it can be unambiguously inferred
	if inFrom && !inTo && direction == "outgoing" {
		return true
	}
	if inTo && !inFrom && direction == "incoming" {
		return true
	}

	return false
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

	lists := GetMapValue(root, "lists")
	if lists != nil && lists.Kind == yaml.MappingNode {
		m.cleanupLists(lists)
	}

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
		if m.isRedundantLabel(field) {
			DeleteMapKey(field, "label")
		}
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
		if m.isRedundantDirection(rel, entityType) {
			DeleteMapKey(rel, "direction")
		}
		if m.isRedundantTargetType(rel, entityType) {
			DeleteMapKey(rel, "target_type")
		}
	}
}

func (m *DataEntryCleanupMigration) cleanupLists(lists *yaml.Node) {
	for i := 1; i < len(lists.Content); i += 2 {
		listDef := lists.Content[i]
		if listDef.Kind != yaml.MappingNode {
			continue
		}

		columns := GetMapValue(listDef, "columns")
		if columns == nil || columns.Kind != yaml.SequenceNode {
			continue
		}
		for _, col := range columns.Content {
			if col.Kind == yaml.MappingNode && m.isRedundantLabel(col) {
				DeleteMapKey(col, "label")
			}
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
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// titleCase converts snake_case or kebab-case to Title Case.
func titleCase(s string) string {
	s = strings.ReplaceAll(s, "_", " ")
	s = strings.ReplaceAll(s, "-", " ")

	words := strings.Fields(s)
	for i, word := range words {
		if word != "" {
			runes := []rune(word)
			runes[0] = unicode.ToUpper(runes[0])
			words[i] = string(runes)
		}
	}

	return strings.Join(words, " ")
}
