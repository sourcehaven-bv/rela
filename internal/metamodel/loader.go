package metamodel

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// validTopLevelKeys are the recognized top-level keys in a metamodel YAML file.
var validTopLevelKeys = map[string]bool{
	"version":     true,
	"namespace":   true,
	"types":       true,
	"entities":    true,
	"relations":   true,
	"validations": true,
	"automations": true,
	"includes":    true,
}

// knownTypos maps common misspellings to the correct key name.
var knownTypos = map[string]string{
	"entity":     "entities",
	"type":       "types",
	"relation":   "relations",
	"validation": "validations",
}

// Load reads and parses a metamodel from a YAML file using the given filesystem.
// If the metamodel contains an `includes:` key, included files are recursively
// loaded and merged. Include paths are resolved relative to the directory
// containing the metamodel file.
//
// The returned []string contains the absolute paths of all files that were
// read: the main metamodel.yaml path plus all include files.
func Load(path string, fs storage.FS) (*Metamodel, []string, error) {
	data, err := fs.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, nil, err
	}

	// When includes are present, parse without full validation first,
	// resolve includes, then validate the merged result.
	m, err := parseRaw(data)
	if err != nil {
		return nil, nil, err
	}

	if len(m.Includes) > 0 {
		rootDir := filepath.Dir(path)
		includePaths, err := loadWithIncludes(m, path, rootDir, fs)
		if err != nil {
			return nil, nil, err
		}
		// Validate the fully merged metamodel
		if err := validate(m); err != nil {
			return nil, nil, err
		}
		sourceFiles := append([]string{absPath}, includePaths...)
		return m, sourceFiles, nil
	}

	// No includes: validate immediately
	if err := validate(m); err != nil {
		return nil, nil, err
	}

	return m, []string{absPath}, nil
}

// LoadWithoutMigrationCheck loads a metamodel without checking for migrations.
// This is used by the migrate command itself to avoid chicken-and-egg issues.
// Returns nil if loading fails (caller should handle gracefully).
//
// The returned []string contains the absolute paths of all files that were read.
func LoadWithoutMigrationCheck(path string, fs storage.FS) (*Metamodel, []string, error) {
	data, err := fs.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, nil, err
	}

	m, err := parseRaw(data)
	if err != nil {
		return nil, nil, err
	}

	if len(m.Includes) > 0 {
		rootDir := filepath.Dir(path)
		includePaths, err := loadWithIncludes(m, path, rootDir, fs)
		if err != nil {
			return nil, nil, err
		}
		sourceFiles := append([]string{absPath}, includePaths...)
		// Skip validation since metamodel may be in a migration state
		return m, sourceFiles, nil
	}

	// Skip validation since metamodel may be in a migration state
	return m, []string{absPath}, nil
}

// Parse parses and validates metamodel YAML content.
func Parse(data []byte) (*Metamodel, error) {
	m, err := parseRaw(data)
	if err != nil {
		return nil, err
	}
	if err := validate(m); err != nil {
		return nil, err
	}
	return m, nil
}

// parseRaw parses metamodel YAML content without semantic validation.
// It performs only structural checks (YAML syntax, unknown keys, reserved types).
func parseRaw(data []byte) (*Metamodel, error) {
	var m Metamodel
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, humanizeYAMLError(err)
	}

	// Check for unknown/misspelled top-level keys
	if err := checkUnknownKeys(data); err != nil {
		return nil, err
	}

	// Validate custom type names don't conflict with built-in types
	for typeName := range m.Types {
		if IsBuiltinType(typeName) {
			return nil, &ReservedTypeNameError{TypeName: typeName}
		}
	}

	// Extract property order from YAML (maps lose key order during unmarshaling)
	if err := extractPropertyOrder(data, &m); err != nil {
		return nil, err
	}

	return &m, nil
}

// extractPropertyOrder parses the YAML using yaml.Node to extract property key order
// for each entity definition. This allows WriteEntity to output properties in the
// same order as defined in the metamodel.
func extractPropertyOrder(data []byte, m *Metamodel) error {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("parse yaml.Node for property order: %w", err)
	}

	// root is a document node, get its content
	if root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
		return nil
	}
	doc := root.Content[0]
	if doc.Kind != yaml.MappingNode {
		return nil
	}

	// Find the "entities" key
	for i := 0; i < len(doc.Content)-1; i += 2 {
		keyNode := doc.Content[i]
		valueNode := doc.Content[i+1]
		if keyNode.Value == "entities" && valueNode.Kind == yaml.MappingNode {
			extractEntityPropertyOrder(valueNode, m)
			break
		}
	}
	return nil
}

// extractEntityPropertyOrder extracts property order from the entities mapping node.
func extractEntityPropertyOrder(entitiesNode *yaml.Node, m *Metamodel) {
	// Iterate over entity definitions
	for i := 0; i < len(entitiesNode.Content)-1; i += 2 {
		entityNameNode := entitiesNode.Content[i]
		entityDefNode := entitiesNode.Content[i+1]

		entityName := entityNameNode.Value
		entityDef, ok := m.Entities[entityName]
		if !ok || entityDefNode.Kind != yaml.MappingNode {
			continue
		}

		// Find the "properties" key within this entity definition
		for j := 0; j < len(entityDefNode.Content)-1; j += 2 {
			keyNode := entityDefNode.Content[j]
			valueNode := entityDefNode.Content[j+1]
			if keyNode.Value == "properties" && valueNode.Kind == yaml.MappingNode {
				// Extract property names in order
				order := make([]string, 0, (len(valueNode.Content)-1)/2+1)
				for k := 0; k < len(valueNode.Content)-1; k += 2 {
					propNameNode := valueNode.Content[k]
					order = append(order, propNameNode.Value)
				}
				entityDef.PropertyOrder = order
				m.Entities[entityName] = entityDef
				break
			}
		}
	}
}

// validate performs structural and semantic validation on a fully assembled metamodel.
func validate(m *Metamodel) error {
	// Validate entity definitions (returns hard errors for structural issues)
	if err := validateEntityStructure(m); err != nil {
		return err
	}

	// Collect semantic validation errors so users see all problems at once
	var validationErrors []string

	if len(m.Entities) == 0 {
		validationErrors = append(validationErrors, "metamodel has no entity types defined")
	}

	validationErrors = append(validationErrors, validateCustomTypes(m)...)
	validationErrors = append(validationErrors, validateEntitySemantics(m)...)
	validationErrors = append(validationErrors, validateRelationReferences(m)...)
	validationErrors = append(validationErrors, validateRelationProperties(m)...)
	validationErrors = append(validationErrors, validateRelationInverses(m)...)
	validationErrors = append(validationErrors, validateRelationOrderable(m)...)

	if len(validationErrors) > 0 {
		return &SchemaValidationError{Errors: validationErrors}
	}

	return nil
}

// validateEntityStructure checks for hard structural errors in entity definitions
// (reserved names, whitespace, conflicting IDs) and builds the alias map.
// Returns immediately on the first error found.
func validateEntityStructure(m *Metamodel) error {
	m.aliasMap = make(map[string]string)

	for name, def := range m.Entities {
		if def.IDType != "" && def.IDType != IDTypeShort && def.IDType != IDTypeSequential && def.IDType != IDTypeManual {
			return &InvalidIDTypeError{EntityType: name, IDType: def.IDType}
		}
		if def.IDCaps != "" && def.IDCaps != IDCapsUpper && def.IDCaps != IDCapsLower {
			return &InvalidIDCapsError{EntityType: name, IDCaps: def.IDCaps}
		}

		for propName := range def.Properties {
			trimmedName := strings.TrimSpace(propName)
			if trimmedName != propName {
				return &WhitespacePropertyError{EntityType: name, PropertyName: propName}
			}
			if ReservedPropertyNames[propName] {
				return &ReservedPropertyError{EntityType: name, PropertyName: propName}
			}
		}

		if def.IDPrefix != "" && len(def.IDPrefixes) > 0 {
			return &ConflictingIDPrefixError{EntityType: name}
		}
		for _, prefix := range def.GetIDPrefixes() {
			if err := ValidateIDPrefix(prefix); err != nil {
				return &InvalidIDPrefixError{EntityType: name, Prefix: prefix, Reason: err.Error()}
			}
		}

		m.aliasMap[strings.ToLower(name)] = name
		for _, alias := range def.Aliases {
			m.aliasMap[strings.ToLower(alias)] = name
		}
	}

	return nil
}

// validateEntitySemantics collects semantic warnings/errors about entity definitions
// (missing labels, properties, ID prefixes, unknown types).
func validateEntitySemantics(m *Metamodel) []string {
	var errs []string

	entityNames := sortedKeys(m.Entities)
	for _, name := range entityNames {
		def := m.Entities[name]

		if def.Label == "" {
			errs = append(errs, fmt.Sprintf("entity %q: missing 'label'", name))
		}
		if len(def.Properties) == 0 {
			errs = append(errs, fmt.Sprintf("entity %q: no properties defined", name))
		}
		idType := def.GetIDType()
		if (idType == IDTypeSequential || idType == IDTypeShort) && def.IDPrefix == "" && len(def.IDPrefixes) == 0 {
			errs = append(errs, fmt.Sprintf(
				"entity %q: no ID prefix defined (set 'id_prefix' or 'id_prefixes', or use 'id_type: manual')", name))
		}
		if def.IDCaps != "" && def.GetIDType() != IDTypeShort {
			errs = append(errs, fmt.Sprintf(
				"entity %q: 'id_caps' has no effect (only applies to 'id_type: short')", name))
		}

		errs = append(errs, validatePropertyDefs(fmt.Sprintf("entity %q", name), def.Properties, m, nil)...)

		errs = append(errs, validateDefaultSort(name, def)...)

		errs = append(errs, validateDisplayProperty(name, def)...)
	}

	return errs
}

// validateDisplayProperty enforces the contract on EntityDef.DisplayProperty:
// when set, the value must (a) have no leading/trailing whitespace,
// (b) reference a defined property on the entity, (c) not be list-typed,
// and (d) be of a scalar type that renders meaningfully as a display
// name (string, integer, boolean, enum). Empty (omitted, null, or "")
// is allowed — GetPrimaryProperty falls back to the autoderivation.
//
// Errors accumulate so the author sees every problem in one load
// (matches the validator's accumulating style). Both the whitespace
// and the missing-property diagnostics list the available property
// names so the fix is obvious.
//
// Type restriction rationale: date/file/rrule values surface in YAML
// frontmatter as time.Time / structured shapes that don't have a
// useful default string rendering ("2026-04-25 00:00:00 +0000 UTC" is
// rarely what an author wants). list-typed properties render lists
// like "[a b c]" or "[]". Restrict at load time so the runtime
// fallback in DisplayTitle stays simple. See review-responses RR-AVOMV,
// RR-IG4JJ, RR-MPE9Y, RR-KTWG9.
func validateDisplayProperty(entityName string, def EntityDef) []string {
	dp := def.DisplayProperty
	if dp == "" {
		return nil
	}

	available := strings.Join(sortedKeys(def.Properties), ", ")

	var errs []string
	if dp != strings.TrimSpace(dp) {
		errs = append(errs, fmt.Sprintf(
			"entity %q: display_property %q has leading or trailing whitespace (have: %s)",
			entityName, dp, available))
		// Don't continue with property-existence/type checks on a
		// value whose user-meant form we can't be sure of.
		return errs
	}

	prop, ok := def.Properties[dp]
	if !ok {
		errs = append(errs, fmt.Sprintf(
			"entity %q: display_property %q is not a defined property (have: %s)",
			entityName, dp, available))
		return errs
	}

	if prop.List {
		errs = append(errs, fmt.Sprintf(
			"entity %q: display_property %q is list-typed; lists cannot render as a display name",
			entityName, dp))
	}

	// Allow string (default), integer, boolean, enum, custom enum-like
	// types defined elsewhere. Reject the structured types whose default
	// rendering is unhelpful.
	switch prop.Type {
	case PropertyTypeDate, PropertyTypeFile, PropertyTypeRrule:
		errs = append(errs, fmt.Sprintf(
			"entity %q: display_property %q has type %q; only string, integer, boolean, or enum types render as display names",
			entityName, dp, prop.Type))
	}

	return errs
}

// validateDefaultSort checks default_sort entries for an entity definition.
func validateDefaultSort(entityName string, def EntityDef) []string {
	var errs []string
	for i, ss := range def.DefaultSort {
		if ss.Property == "" {
			errs = append(errs, fmt.Sprintf(
				"entity %q: default_sort[%d] has no property specified", entityName, i))
			continue
		}
		// "id" and "modified" are virtual sort properties
		if ss.Property != "id" && ss.Property != "modified" {
			if _, ok := def.Properties[ss.Property]; !ok {
				errs = append(errs, fmt.Sprintf(
					"entity %q: default_sort references unknown property %q", entityName, ss.Property))
			}
		}
		if ss.Direction != "" && ss.Direction != "asc" && ss.Direction != "desc" {
			errs = append(errs, fmt.Sprintf(
				"entity %q: default_sort[%d] has invalid direction %q (use \"asc\" or \"desc\")",
				entityName, i, ss.Direction))
		}
	}
	return errs
}

// validateCustomTypes validates custom type definitions, compiles regex patterns,
// and stores the compiled regexes for use during validation.
func validateCustomTypes(m *Metamodel) []string {
	var errs []string

	typeNames := sortedKeys(m.Types)
	for _, typeName := range typeNames {
		customType := m.Types[typeName]

		for i := range customType.Validations {
			validation := &customType.Validations[i]

			if validation.Pattern == "" {
				errs = append(errs, fmt.Sprintf(
					"type %q: validation[%d] has empty pattern", typeName, i))
				continue
			}
			if validation.Error == "" {
				errs = append(errs, fmt.Sprintf(
					"type %q: validation[%d] has empty error message", typeName, i))
			}

			re, err := regexp.Compile(validation.Pattern)
			if err != nil {
				errs = append(errs, fmt.Sprintf(
					"type %q: validation[%d] has invalid regex pattern %q: %v",
					typeName, i, validation.Pattern, err))
			} else {
				// Cache the compiled regex for use during validation
				validation.SetCompiled(re)
			}
		}

		// Write back the modified type with compiled regexes
		m.Types[typeName] = customType
	}

	return errs
}

// validateRelationReferences checks that all entity types referenced in relations exist.
func validateRelationReferences(m *Metamodel) []string {
	var errs []string

	relNames := sortedKeys(m.Relations)
	for _, name := range relNames {
		rel := m.Relations[name]
		// A relation with no 'from' or no 'to' types is meaningless: no
		// entity can ever be a valid source/target, so any cardinality
		// constraint on it is a silent no-op. Reject at load (likely a
		// typo or an omitted field) rather than letting it pass quietly.
		if len(rel.From) == 0 {
			errs = append(errs, fmt.Sprintf(
				"relation %q: must declare at least one 'from' entity type", name))
		}
		if len(rel.To) == 0 {
			errs = append(errs, fmt.Sprintf(
				"relation %q: must declare at least one 'to' entity type", name))
		}
		for _, fromType := range rel.From {
			if _, ok := m.Entities[fromType]; !ok {
				errs = append(errs, fmt.Sprintf(
					"relation %q: references unknown entity type %q in 'from'", name, fromType))
			}
		}
		for _, toType := range rel.To {
			if _, ok := m.Entities[toType]; !ok {
				errs = append(errs, fmt.Sprintf(
					"relation %q: references unknown entity type %q in 'to'", name, toType))
			}
		}
	}

	return errs
}

// validateRelationInverses enforces the cross-relation uniqueness
// rules on `inverse:` declarations and, on success, populates
// `m.inverseOwners` for O(1) runtime lookup.
//
// Two failure modes are rejected:
//
//   - Two unrelated canonical relations declare the same `inverse:` ID.
//     Without this guard, a consumer that resolves a body key by
//     inverse name would pick non-deterministically across runs
//     (Go map iteration is randomized).
//   - A relation declares `inverse: X` where `X` is also the name of
//     a separate canonical relation. The lookup precedence would be
//     ambiguous: canonical first wins by convention, but the
//     metamodel author likely didn't intend the shadowing.
//
// Symmetric self-inverse (`symmetric: true` AND `inverse.id == relType`)
// is the one allowed case where a name appears in both maps — it
// describes a single relation that is its own inverse.
//
// If any violation is found, `inverseOwners` is left nil so callers
// surface a clear "metamodel did not pass validation" failure rather
// than reading a partially populated map.
func validateRelationInverses(m *Metamodel) []string {
	var errs []string
	owners := make(map[string]string, len(m.Relations))

	for _, relType := range sortedKeys(m.Relations) {
		rel := m.Relations[relType]
		if rel.Inverse == nil || rel.Inverse.ID == "" {
			continue
		}
		inv := rel.Inverse.ID

		// Symmetric self-inverse is the only allowed name overlap.
		isSelfSymmetric := rel.Symmetric && inv == relType

		if existing, ok := owners[inv]; ok {
			errs = append(errs, fmt.Sprintf(
				"inverse_name_collision: relations %q and %q both declare inverse %q "+
					"(each inverse name must be unique across the metamodel; "+
					"rename one of the `inverse:` values or remove the duplicate)",
				existing, relType, inv))
			continue
		}

		if _, shadowsCanonical := m.Relations[inv]; shadowsCanonical && !isSelfSymmetric {
			errs = append(errs, fmt.Sprintf(
				"inverse_shadows_canonical: relation %q declares inverse %q which is also the name of canonical relation %q "+
					"(rename the inverse to a unique name; "+
					"for a self-inverse, set `symmetric: true` on the canonical relation and use its own name as inverse)",
				relType, inv, inv))
			continue
		}

		owners[inv] = relType
	}

	if len(errs) == 0 {
		m.inverseOwners = owners
	}
	return errs
}

// validateRelationProperties validates property definitions on relation types.
// Reserved property names for relations are: from, relation, to (used in YAML frontmatter).
func validateRelationProperties(m *Metamodel) []string {
	errs := make([]string, 0)

	// Reserved property names for relations
	reservedRelProps := map[string]bool{
		"from":     true,
		"relation": true,
		"to":       true,
	}

	relNames := sortedKeys(m.Relations)
	for _, name := range relNames {
		rel := m.Relations[name]
		errs = append(errs, validatePropertyDefs(fmt.Sprintf("relation %q", name), rel.Properties, m, reservedRelProps)...)
		// Forbid users from declaring the managed order properties explicitly:
		// rela owns these names, and a user-supplied PropertyDef would conflict
		// with the auto-assigned float values written by the entity manager.
		if _, ok := rel.Properties[OrderPropertyOut]; ok {
			errs = append(errs, fmt.Sprintf(
				"relation %q: property %q is managed by rela and cannot be declared", name, OrderPropertyOut))
		}
		if _, ok := rel.Properties[OrderPropertyIn]; ok {
			errs = append(errs, fmt.Sprintf(
				"relation %q: property %q is managed by rela and cannot be declared", name, OrderPropertyIn))
		}
	}

	return errs
}

// validateRelationOrderable rejects relation types that declare an Orderable
// value outside the allowed enum, or that combine Orderable with Symmetric
// (which has no meaningful semantics — a symmetric relation has only one
// edge between any pair of entities, so "ordering" is undefined).
func validateRelationOrderable(m *Metamodel) []string {
	var errs []string

	for _, name := range sortedKeys(m.Relations) {
		rel := m.Relations[name]
		if !rel.Orderable.IsValid() {
			errs = append(errs, fmt.Sprintf(
				"relation %q: invalid orderable value %q (allowed: outgoing, incoming, both)",
				name, string(rel.Orderable)))
			continue
		}
		if rel.Orderable != OrderableNone && rel.Symmetric {
			errs = append(errs, fmt.Sprintf(
				"relation %q: orderable cannot be combined with symmetric — symmetric relations have no canonical direction to order",
				name))
		}
	}

	return errs
}

// sortedKeys returns the keys of a map sorted alphabetically.
// Works with any map type using a generic constraint would be ideal,
// but we use interface{} maps here.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// validatePropertyDefs validates property definitions for entities or relations.
// schemaName is used in error messages (e.g., "entity \"foo\"" or "relation \"bar\"").
// reserved is an optional set of reserved property names (nil for entities).
func validatePropertyDefs(
	schemaName string, props map[string]PropertyDef, m *Metamodel, reserved map[string]bool,
) []string {
	var errs []string

	for propName, propDef := range props {
		// Check for reserved property names
		if reserved != nil && reserved[propName] {
			errs = append(errs, fmt.Sprintf(
				"%s: property %q is reserved and cannot be used", schemaName, propName))
			continue
		}

		// Check property type is specified
		if propDef.Type == "" {
			errs = append(errs, fmt.Sprintf(
				"%s: property %q has no type specified", schemaName, propName))
			continue
		}

		// Check property type is known
		if !isKnownPropertyType(propDef.Type, m) {
			if propDef.Type == "number" || propDef.Type == "float" {
				errs = append(errs, fmt.Sprintf(
					"%s: property %q has type %q which is not supported; use \"integer\" instead",
					schemaName, propName, propDef.Type))
			} else {
				errs = append(errs, fmt.Sprintf(
					"%s: property %q has unknown type %q (not a built-in type and not defined in 'types')",
					schemaName, propName, propDef.Type))
			}
		}

		// Check enum has values
		if propDef.Type == PropertyTypeEnum && len(propDef.Values) == 0 {
			errs = append(errs, fmt.Sprintf(
				"%s: property %q is type \"enum\" but has no 'values' list", schemaName, propName))
		}

		errs = append(errs, validateFilePropertyOptions(schemaName, propName, propDef)...)
	}

	return errs
}

// validateFilePropertyOptions checks the attachment-only property options
// (`max`, `accept`, `scan`, `scan_cmd`, `transform`): `max` must be >= 1, and
// none of these may appear on a non-`file` property.
func validateFilePropertyOptions(schemaName, propName string, propDef PropertyDef) []string {
	var errs []string
	if propDef.Max < 0 {
		errs = append(errs, fmt.Sprintf(
			"%s: property %q has max %d; must be >= 1", schemaName, propName, propDef.Max))
	}
	if propDef.Type == PropertyTypeFile {
		return errs
	}
	// Below here the property is NOT a file: none of the attachment options apply.
	if propDef.Max != 0 {
		errs = append(errs, fileOnlyOptionErr(schemaName, propName, "max", propDef.Type))
	}
	if len(propDef.Accept) > 0 {
		errs = append(errs, fileOnlyOptionErr(schemaName, propName, "accept", propDef.Type))
	}
	if propDef.Scan != ScanDefault {
		errs = append(errs, fileOnlyOptionErr(schemaName, propName, "scan", propDef.Type))
	}
	if len(propDef.ScanCmd) > 0 || len(propDef.Transform) > 0 {
		errs = append(errs, fileOnlyOptionErr(schemaName, propName, "scan_cmd/transform", propDef.Type))
	}
	return errs
}

func fileOnlyOptionErr(schemaName, propName, option, gotType string) string {
	return fmt.Sprintf("%s: property %q sets %q but is type %q; only applies to type \"file\"",
		schemaName, propName, option, gotType)
}

// isKnownPropertyType checks if a property type is valid (built-in, legacy, or custom).
func isKnownPropertyType(typeName string, m *Metamodel) bool {
	if IsBuiltinType(typeName) {
		return true
	}
	// Legacy built-in types
	if typeName == "status" || typeName == "priority" {
		return true
	}
	// Custom types
	_, ok := m.Types[typeName]
	return ok
}

// checkUnknownKeys detects unknown top-level keys in the metamodel YAML.
// This catches common typos like "entity" instead of "entities".
func checkUnknownKeys(data []byte) error {
	var raw map[string]interface{}
	if unmarshalErr := yaml.Unmarshal(data, &raw); unmarshalErr != nil {
		// If we can't unmarshal as a map, the struct unmarshal already failed
		// with a better error, so skip this check
		return nil //nolint:nilerr // intentional: struct unmarshal error is better
	}

	var unknownKeyErrors []string
	for key := range raw {
		if validTopLevelKeys[key] {
			continue
		}
		if suggestion, ok := knownTypos[key]; ok {
			unknownKeyErrors = append(unknownKeyErrors,
				fmt.Sprintf("unknown key %q (did you mean %q?)", key, suggestion))
		} else {
			keys := make([]string, 0, len(validTopLevelKeys))
			for k := range validTopLevelKeys {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			unknownKeyErrors = append(unknownKeyErrors,
				fmt.Sprintf("unknown key %q (valid keys: %s)", key, strings.Join(keys, ", ")))
		}
	}

	if len(unknownKeyErrors) > 0 {
		sort.Strings(unknownKeyErrors)
		return &SchemaValidationError{Errors: unknownKeyErrors}
	}
	return nil
}

// DefaultMetamodel returns a minimal default metamodel
func DefaultMetamodel() *Metamodel {
	return &Metamodel{
		Version:   "1.0",
		Namespace: "https://example.org/ontology/architecture#",
		Types: map[string]CustomType{
			"status": {
				Values:  []string{"draft", "proposed", "accepted", "deprecated", "rejected", "retired"},
				Default: "draft",
			},
			"priority": {
				Values: []string{"critical", "high", "medium", "low"},
			},
		},
		Entities: map[string]EntityDef{
			"requirement": {
				Label:    "Requirement",
				Aliases:  []string{"req"},
				IDPrefix: "REQ-",
				Properties: map[string]PropertyDef{
					"title":       {Type: "string", Required: true},
					"description": {Type: "string"},
					"status":      {Type: "status", Required: true},
					"priority":    {Type: "priority"},
				},
			},
			"decision": {
				Label:      "Decision",
				Aliases:    []string{"dec", "adr"},
				IDPrefixes: []string{"DEC-", "ADR-"},
				Properties: map[string]PropertyDef{
					"title":     {Type: "string", Required: true},
					"rationale": {Type: "string"},
					"status":    {Type: "status", Required: true},
				},
			},
			"solution": {
				Label:    "Solution",
				Aliases:  []string{"sol"},
				IDPrefix: "SOL-",
				Properties: map[string]PropertyDef{
					"title":       {Type: "string", Required: true},
					"description": {Type: "string"},
					"status":      {Type: "status"},
				},
			},
			"component": {
				Label:      "Component",
				Aliases:    []string{"comp"},
				IDPrefixes: []string{"COMP-", "AC-", "TC-"},
				Properties: map[string]PropertyDef{
					"title": {Type: "string", Required: true},
				},
			},
		},
		Relations: map[string]RelationDef{
			"addresses": {
				Label:       "addresses",
				Description: "A decision addresses a requirement",
				From:        []string{"decision"},
				To:          []string{"requirement"},
				Inverse:     &InverseDef{ID: "addressedBy"},
			},
			"implements": {
				Label:       "implements",
				Description: "A solution implements a decision",
				From:        []string{"solution"},
				To:          []string{"decision"},
				Inverse:     &InverseDef{ID: "implementedBy"},
			},
			"realizes": {
				Label:       "realizes",
				Description: "A component realizes a solution",
				From:        []string{"component"},
				To:          []string{"solution"},
				Inverse:     &InverseDef{ID: "realizedBy"},
			},
			"dependsOn": {
				Label:   "depends on",
				From:    []string{"component", "solution", "decision"},
				To:      []string{"component", "solution", "decision"},
				Inverse: &InverseDef{ID: "dependencyOf"},
			},
		},
		aliasMap: make(map[string]string),
	}
}

// DefaultMetamodelYAML returns the default metamodel as YAML
func DefaultMetamodelYAML() string {
	return `# Architecture Metamodel
# This file defines the entity types, relations, and validation rules for your project.

version: "1.0"
namespace: "https://example.org/ontology/architecture#"

# Custom enum types (reusable across entities)
types:
  status:
    values: [draft, proposed, accepted, deprecated, rejected, retired]
    default: draft

  priority:
    values: [critical, high, medium, low]

# Entity type definitions
entities:
  requirement:
    label: Requirement
    aliases: [req]
    id_prefix: "REQ-"
    properties:
      title:
        type: string
        required: true
      description:
        type: string
      status:
        type: status
        required: true
      priority:
        type: priority

  decision:
    label: Decision
    aliases: [dec, adr]
    id_prefixes: ["DEC-", "ADR-"]
    properties:
      title:
        type: string
        required: true
      rationale:
        type: string
      status:
        type: status
        required: true

  solution:
    label: Solution
    aliases: [sol]
    id_prefix: "SOL-"
    properties:
      title:
        type: string
        required: true
      description:
        type: string
      status:
        type: status

  component:
    label: Component
    aliases: [comp]
    id_prefixes: ["COMP-", "AC-", "TC-"]
    properties:
      title:
        type: string
        required: true

# Relation definitions
relations:
  addresses:
    label: addresses
    description: A decision addresses a requirement
    from: [decision]
    to: [requirement]
    inverse: addressedBy

  implements:
    label: implements
    description: A solution implements a decision
    from: [solution]
    to: [decision]
    inverse: implementedBy

  realizes:
    label: realizes
    description: A component realizes a solution
    from: [component]
    to: [solution]
    inverse: realizedBy

  dependsOn:
    label: depends on
    from: [component, solution, decision]
    to: [component, solution, decision]
    inverse: dependencyOf

# Custom validation rules (optional)
# Define rules to check entity properties using filter expressions.
# Uses the same syntax as --where filters: =, !=, <, <=, >, >=, =~ (regex)
#
# validations:
#   - name: accepted-requirements-need-priority
#     description: "Accepted requirements must have a priority assigned"
#     entity_type: requirement
#     when:                        # IF these conditions match...
#       - "status=accepted"
#     then:                        # THEN these must be true
#       - "priority!="
#     severity: error
#
#   - name: decisions-need-rationale
#     description: "All decisions should have a rationale"
#     entity_type: decision
#     then:
#       - "rationale!="
#     severity: warning
`
}
