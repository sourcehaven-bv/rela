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
	"description": true,
	"types":       true,
	"entities":    true,
	"relations":   true,
	"validations": true,
	"automations": true,
	"includes":    true,
	"attachments": true,
	"transforms":  true,
	"worlds":      true,
	"copies":      true,
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
	validationErrors = append(validationErrors, validateRelationScope(m)...)
	validationErrors = append(validationErrors, validateTransforms(m)...)
	validationErrors = append(validationErrors, validateCopies(m)...)
	validationErrors = append(validationErrors, validateFaces(m)...)
	validationErrors = append(validationErrors, validateWorlds(m)...)
	validationErrors = append(validationErrors, validateValidationFaces(m)...)
	validationErrors = append(validationErrors, validateAutomationFaces(m)...)

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

		// The entity-type name is interpolated into backend DDL by the
		// derived-schema reconciler (`... WHERE type = '<type>'`), so it must
		// use the safe character set for the same DDL-injection reason as
		// property names (TKT-3Q0GP1).
		if err := ValidateSchemaName(name); err != nil {
			errs = append(errs, fmt.Sprintf("entity %v", err))
		}

		// "@" additionally separates a type from a content state in an ACL
		// write grant (`page@draft`, TKT-DN37J2). A type name carrying one
		// makes that grant ambiguous in BOTH directions: a grant on a type
		// named `a@b` stops matching it, and starts matching a different
		// type named `a`. The second is a cross-type privilege escalation.
		//
		// Rejected here rather than in ValidateSchemaName because that is
		// shared with property names, which are not grant subjects and have
		// no reason to lose a legal character.
		if strings.Contains(name, "@") {
			errs = append(errs, fmt.Sprintf(
				"entity %q: entity type names must not contain '@' — it separates "+
					"a type from a content state in ACL write grants (page@draft), "+
					"so a type name carrying one makes those grants ambiguous", name))
		}

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

		errs = append(errs, validatePropertyDefs(fmt.Sprintf("entity %q", name), def.Properties, m, nil, true)...)

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

	// A template (contains `{`) references properties via `{name}`
	// placeholders. Each named property is checked exactly as a bare-name
	// display_property would be.
	if isDisplayTemplate(dp) {
		names, err := parseDisplayTemplate(dp)
		if err != nil {
			return append(errs, fmt.Sprintf("entity %q: display_property: %s", entityName, err))
		}
		for _, name := range names {
			errs = append(errs, validateDisplayPropertyRef(entityName, dp, name, def.Properties, available)...)
		}
		return errs
	}

	return append(errs, validateDisplayPropertyRef(entityName, dp, dp, def.Properties, available)...)
}

// validateDisplayPropertyRef checks that a single property referenced by a
// display_property (bare name or template placeholder) exists and has a type
// that renders meaningfully as a display name. dp is the whole
// display_property value (for the error message); name is the referenced
// property. See validateDisplayProperty for the type-restriction rationale.
func validateDisplayPropertyRef(
	entityName, dp, name string, props map[string]PropertyDef, available string,
) []string {
	prop, ok := props[name]
	if !ok {
		return []string{fmt.Sprintf(
			"entity %q: display_property %q references undefined property %q (have: %s)",
			entityName, dp, name, available)}
	}

	var errs []string
	if prop.List {
		errs = append(errs, fmt.Sprintf(
			"entity %q: display_property %q references list-typed property %q; lists cannot render as a display name",
			entityName, dp, name))
	}

	// Allow string (default), integer, boolean, enum, custom enum-like
	// types defined elsewhere. Reject the structured types whose default
	// rendering is unhelpful.
	switch prop.Type {
	case PropertyTypeDate, PropertyTypeDatetime, PropertyTypeFile, PropertyTypeRrule:
		errs = append(errs, fmt.Sprintf(
			"entity %q: display_property %q references property %q of type %q; "+
				"only string, integer, boolean, or enum types render as display names",
			entityName, dp, name, prop.Type))
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
		errs = append(errs, validatePropertyDefs(fmt.Sprintf("relation %q", name), rel.Properties, m, reservedRelProps, false)...)
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

// validateRelationScope rejects unknown `scope:` values on relation
// types (TKT-DOFYR1). Absent means identity — the safe default that
// keeps a faceless project byte-identical.
func validateRelationScope(m *Metamodel) []string {
	var errs []string
	for _, name := range sortedKeys(m.Relations) {
		rel := m.Relations[name]
		if !rel.Scope.IsValid() {
			errs = append(errs, fmt.Sprintf(
				"relation %q: invalid scope value %q (allowed: identity, content)",
				name, string(rel.Scope)))
		}
	}
	return errs
}

// validateFaces checks the per-type `faces:` declarations
// (TKT-WAV8XP, design doc §4.1): at most one `bare_face` per type.
//
// The face NAME grammar is deliberately NOT checked here. It belongs
// to [github.com/Sourcehaven-BV/rela/internal/entity].ParseFace, and
// this package must not import entity (arch-lint keeps entity a leaf);
// internal/worlds applies it when compiling. See that package's Compile.
func validateFaces(m *Metamodel) []string {
	var errs []string
	for _, typeName := range sortedKeys(m.Entities) {
		def := m.Entities[typeName]
		// `bare_face` must name a face this type declares. A typo would
		// otherwise leave the bare-id row unnamed while the face the operator
		// meant became a separate suffixed row — two rows where they intended
		// one, and no error to say so.
		//
		// The old spelling was `bare_face` on each face, which needed a
		// second check that at most one claimed it. A single key on the type
		// cannot be set twice, so that check is gone rather than moved.
		if def.BareFace == "" {
			continue
		}
		if len(def.Faces) == 0 {
			errs = append(errs, fmt.Sprintf(
				"entity %q: `bare_face: %s` but the type declares no `faces:` — "+
					"a type without faces has exactly one state and needs no name for it",
				typeName, def.BareFace))
			continue
		}
		if _, ok := def.Faces[def.BareFace]; !ok {
			errs = append(errs, fmt.Sprintf(
				"entity %q: `bare_face: %s` names no declared face (declares: %s)",
				typeName, def.BareFace, strings.Join(sortedKeys(def.Faces), ", ")))
		}
	}
	return errs
}

// validateWorlds checks the `worlds:` declarations (TKT-WAV8XP, design
// doc §4.1). Every check here is a LOAD-TIME refusal, not a warning:
// a world that resolves wrongly shows the wrong face of a document, and
// the whole feature exists to make that impossible.
//
// The load-bearing one is the mandatory `otherwise:` — a silent fallback
// is exactly the leak this feature prevents (a `published` world quietly
// showing a draft face), so the absent value is rejected rather than
// defaulted in either direction.
func validateWorlds(m *Metamodel) []string {
	var errs []string
	for _, worldName := range sortedKeys(m.Worlds) {
		world := m.Worlds[worldName]
		if strings.EqualFold(worldName, DefaultWorldName) {
			// Case-folded: YAML keys are case-sensitive but humans are not,
			// and a `Default:` world that silently coexists with the implicit
			// one is a confusion with no upside.
			errs = append(errs, fmt.Sprintf(
				"world %q: the name is reserved — the default world is implicit and total "+
					"(every entity contributes its default state) and cannot be redeclared",
				worldName))
		}
		if err := ValidateSchemaName(worldName); err != nil {
			// The empty name is the one THIS check catches that matters: it
			// would make a lookup with an unpopulated name succeed and
			// return a real, non-default world — the inverse of the
			// fail-closed rule.
			//
			// It is NOT the whole story, and this comment used to imply it
			// was. ValidateSchemaName is a blocklist tuned for type and
			// property names, so it still admits `/`, `%`, `?`, `&`, `#`,
			// `..` and internal spaces — all hostile in the `?world=` /
			// `--world` / `acl.yaml` contexts a world name reaches. The
			// strict allowlist lives in internal/worlds.validateWorldNames,
			// because that grammar is entity.ParseFace's and
			// internal/metamodel may not import internal/entity (arch-lint).
			// Both run at load; this one first, for the better message on a
			// blank name.
			errs = append(errs, fmt.Sprintf("world %q: invalid name: %v", worldName, err))
		}
		if len(world.Select) == 0 && len(world.Overrides) == 0 {
			// Forgetting `select:` (or slipping its indentation) is a likelier
			// typo than a bad face name, and it resolves EVERY faced
			// entity through `otherwise:` alone — a world that shows nothing.
			// It fails safe but silently, so it is rejected rather than left
			// to be diagnosed as "the published site is empty".
			errs = append(errs, fmt.Sprintf(
				"world %q: declares neither `select:` nor `overrides:` — every entity whose type "+
					"declares faces would resolve through `otherwise:` alone, which is a world "+
					"that shows nothing. Name the state(s) this world selects",
				worldName))
		}
		if !world.Otherwise.IsValid() {
			errs = append(errs, fmt.Sprintf(
				"world %q: `otherwise:` is required and must be %q or %q (got %q) — it decides what "+
					"happens to an entity whose type declares faces the world does not select, and "+
					"defaulting it silently is how a published world ends up showing a draft",
				worldName, OtherwiseExclude, OtherwiseDefault, string(world.Otherwise)))
		}
		errs = append(errs, validateWorldChains(m, worldName, world)...)
		errs = append(errs, validateWorldEdits(m, worldName, world)...)
		errs = append(errs, validateWorldPrimaryFor(m, worldName, world)...)
		errs = append(errs, validateWorldOnAbsent(m, worldName, world)...)
	}
	errs = append(errs, validateFacePrimacy(m)...)
	return errs
}

// validateWorldOnAbsent checks that `on_absent.redirect` names a world a
// reader can be sent to: a declared one, or the implicit default. A redirect
// to this same world would loop; a redirect to an undeclared name would be a
// 400 on arrival — both are load errors, not runtime surprises.
func validateWorldOnAbsent(m *Metamodel, worldName string, world WorldDef) []string {
	target := world.OnAbsent.Redirect
	if target == "" {
		return nil
	}
	if target == worldName {
		return []string{fmt.Sprintf(
			"world %q: `on_absent.redirect` names this world itself, which would redirect forever",
			worldName)}
	}
	if target != DefaultWorldName {
		if _, ok := m.Worlds[target]; !ok {
			return []string{fmt.Sprintf(
				"world %q: `on_absent.redirect` names world %q, which is not declared (declare it, or use %q)",
				worldName, target, DefaultWorldName)}
		}
	}
	return nil
}

// validateWorldPrimaryFor checks that every face a world claims in
// `primary_for:` is one this world actually HEADS (TKT-MFVH03).
//
// The key may only CONFIRM the compiled chains, never contradict them. A world
// declaring itself primary for a face it does not lead would make the
// face-switcher navigate somewhere the face is not primary — the affordance
// that lies. Refusing at load is the same discipline `edits:` and the chain
// names already get: a resolution mistake is not something to discover from a
// wrong-looking page.
func validateWorldPrimaryFor(m *Metamodel, worldName string, world WorldDef) []string {
	var errs []string
	for _, face := range world.PrimaryFor {
		if headsFaceForSomeType(m, world, face) {
			continue
		}
		errs = append(errs, fmt.Sprintf(
			"world %q: `primary_for: %s` names a face this world does not head — "+
				"`primary_for:` breaks a tie between worlds that ALREADY lead a face, "+
				"it cannot make a world primary for one it only falls back to. "+
				"Put %q at the front of `select:` (or of the type's `overrides:` chain) "+
				"if this world should serve it",
			worldName, face, face))
	}
	return errs
}

// headsFaceForSomeType reports whether world leads with face for at least one
// entity type — its own `select:` chain, or any per-type `overrides:` chain.
//
// "For SOME type" rather than "for every type" because `overrides:` makes
// headship per (type, face): a world may head `en` for `guide` via an override
// while its `select:` leads with something else entirely. Claiming primacy is
// legitimate as long as there is a type the claim is true for.
func headsFaceForSomeType(_ *Metamodel, world WorldDef, face string) bool {
	if len(world.Select) > 0 && world.Select[0] == face {
		return true
	}
	for _, chain := range world.Overrides {
		if len(chain) > 0 && chain[0] == face {
			return true
		}
	}
	return false
}

// sortedPrimacyKeys orders the (entityType, face) pairs so a schema with more
// than one ambiguity reports them in a stable order — a load error that
// reshuffles between runs is one an operator cannot diff.
func sortedPrimacyKeys[V any](m map[primacyKey]V) []primacyKey {
	out := make([]primacyKey, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].entityType != out[j].entityType {
			return out[i].entityType < out[j].entityType
		}
		if out[i].face != out[j].face {
			return out[i].face < out[j].face
		}
		return out[i].otherwise < out[j].otherwise
	})
	return out
}

// primacyKey identifies one (entity type, face, otherwise) triple — the
// granularity at which headship is decided. Per type because `overrides:` makes
// the answer per type; per `otherwise:` because two worlds leading the same
// face but resolving its absence differently are distinguishable, and only
// indistinguishable worlds are ambiguous.
type primacyKey struct{ entityType, face, otherwise string }

// validateFacePrimacy rejects an UNDECLARED tie between worlds that are
// INDISTINGUISHABLE for a face (TKT-MFVH03).
//
// Without this the face-switcher answered "which world serves this face" by map
// iteration order — insertion order, a property of how the config serialized
// rather than of what the operator meant, and liable to change when an
// unrelated world was added or renamed. That is the same silent key-order
// failure `bare_face:` was introduced to remove on the face side.
//
// # What counts as a tie, and what deliberately does not
//
// Sharing a chain HEAD is not enough. Two worlds routinely lead the same face
// and differ in `otherwise:`, and that pair is meaningful rather than
// ambiguous: the prototype's `published` (otherwise: exclude — absence is the
// publication bit) and a lenient sibling (otherwise: default — substitute
// instead of vanishing) select the same face and answer a DIFFERENT question
// about the entities that lack it. Rejecting those would fail working schemas
// for a question the operator has already answered.
//
// A tie is therefore two worlds that lead the same face for a type AND resolve
// it identically — same head, same `otherwise:`. Those two genuinely serve the
// reader the same thing, so "which one does the face-switch mean" has no
// answer in the chains, and the operator has to say.
//
// Failing the LOAD rather than picking is deliberate and matches the rest of
// this file: a schema whose resolution is ambiguous is a schema whose author
// has not yet decided, and the decision is cheap to write down.
func validateFacePrimacy(m *Metamodel) []string {
	// (entityType, face) -> worlds heading it, in sorted order for a stable
	// message.
	heads := map[primacyKey][]string{}
	claimed := map[primacyKey][]string{}

	for _, worldName := range sortedKeys(m.Worlds) {
		world := m.Worlds[worldName]
		for _, typeName := range sortedKeys(m.Entities) {
			chain := world.Select
			if o, ok := world.Overrides[typeName]; ok {
				chain = o
			}
			if len(chain) == 0 {
				continue
			}
			// Only a type that DECLARES the face can be served it, so a
			// world's chain head is irrelevant to types that never have it.
			def := m.Entities[typeName]
			if _, declared := def.Faces[chain[0]]; !declared {
				continue
			}
			// Keyed by the RESOLUTION, not just the head: two worlds leading
			// the same face with different `otherwise:` answer different
			// questions and are not interchangeable.
			k := primacyKey{typeName, chain[0], string(world.Otherwise)}
			heads[k] = append(heads[k], worldName)
			for _, claim := range world.PrimaryFor {
				if claim == chain[0] {
					claimed[k] = append(claimed[k], worldName)
				}
			}
		}
	}

	var errs []string
	for _, k := range sortedPrimacyKeys(heads) {
		worlds := heads[k]
		if len(worlds) < 2 {
			continue
		}
		switch len(claimed[k]) {
		case 1:
			// Exactly one world claimed it: the tie is resolved.
		case 0:
			errs = append(errs, fmt.Sprintf(
				"entity %q: worlds %s all lead with face %q, so which one serves it is ambiguous. "+
					"Add `primary_for: [%s]` to the world that should own the face-switch to it",
				k.entityType, strings.Join(worlds, ", "), k.face, k.face))
		default:
			errs = append(errs, fmt.Sprintf(
				"entity %q: worlds %s each claim `primary_for: %s`, but a face has one canonical "+
					"home — remove the claim from all but one",
				k.entityType, strings.Join(claimed[k], ", "), k.face))
		}
	}
	return errs
}

// validateWorldChains checks that every coordinate a world selects — in
// its global chain or a per-type override — is declared by the types it
// can apply to, and that override keys name real entity types.
//
// A world's global `select` naming a coordinate some types do not declare
// is NORMAL, not an error: that is resolution rule 3, which `otherwise:`
// exists to answer. The error is a chain no type at all could satisfy
// (certainly a typo), and an OVERRIDE naming a coordinate its own type
// does not declare (unambiguously a mistake — the override names one type).
func validateWorldChains(m *Metamodel, worldName string, world WorldDef) []string {
	var errs []string
	for _, ptr := range world.Select {
		if !anyTypeDeclaresFace(m, ptr) {
			errs = append(errs, fmt.Sprintf(
				"world %q: `select:` names face %q, which no entity type declares",
				worldName, ptr))
		}
	}
	for _, typeName := range sortedKeys(world.Overrides) {
		def, ok := m.Entities[typeName]
		if !ok {
			errs = append(errs, fmt.Sprintf(
				"world %q: `overrides:` names unknown entity type %q", worldName, typeName))
			continue
		}
		if len(def.Faces) == 0 {
			errs = append(errs, fmt.Sprintf(
				"world %q: `overrides:` names entity type %q, which declares no faces — "+
					"a type without content states contributes its only state to every world, "+
					"so there is nothing to override",
				worldName, typeName))
			continue
		}
		if len(world.Overrides[typeName]) == 0 {
			errs = append(errs, fmt.Sprintf(
				"world %q: `overrides:` gives entity type %q an empty chain — that resolves every "+
					"%s through `otherwise:` alone. Name the state(s), or drop the override to let "+
					"the world's `select:` apply",
				worldName, typeName, typeName))
			continue
		}
		for _, ptr := range world.Overrides[typeName] {
			if _, declared := def.Faces[ptr]; !declared {
				errs = append(errs, fmt.Sprintf(
					"world %q: `overrides:` selects face %q for entity type %q, which declares "+
						"only: %s",
					worldName, ptr, typeName, strings.Join(sortedKeys(def.Faces), ", ")))
			}
		}
	}
	return errs
}

// validateWorldEdits checks the `edits:` target names a declared face.
//
// Step 2 does not USE this field — the copy kernel is Step 4 — but it is
// validated now so a typo surfaces against the schema that introduced it
// rather than a release later.
func validateWorldEdits(m *Metamodel, worldName string, world WorldDef) []string {
	if world.Edits == "" {
		return nil
	}
	if anyTypeDeclaresFace(m, world.Edits) {
		return nil
	}
	return []string{fmt.Sprintf(
		"world %q: `edits:` names face %q, which no entity type declares",
		worldName, world.Edits)}
}

// anyTypeDeclaresFace reports whether any entity type declares ptr.
func anyTypeDeclaresFace(m *Metamodel, ptr string) bool {
	for _, def := range m.Entities {
		if _, ok := def.Faces[ptr]; ok {
			return true
		}
	}
	return false
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
	schemaName string, props map[string]PropertyDef, m *Metamodel, reserved map[string]bool, allowComputed bool,
) []string {
	errs := make([]string, 0, len(props))
	for propName, propDef := range props {
		errs = append(errs, validatePropertyDef(schemaName, propName, propDef, m, reserved, allowComputed)...)
	}
	return errs
}

func validatePropertyDef(
	schemaName, propName string, propDef PropertyDef, m *Metamodel, reserved map[string]bool, allowComputed bool,
) []string {
	if reserved != nil && reserved[propName] {
		return []string{fmt.Sprintf("%s: property %q is reserved and cannot be used", schemaName, propName)}
	}
	// Property names reach backend DDL, so validate the safe character set at load.
	if err := ValidateSchemaName(propName); err != nil {
		return []string{fmt.Sprintf("%s: property %v", schemaName, err)}
	}
	if propDef.Type == "" {
		return []string{fmt.Sprintf("%s: property %q has no type specified", schemaName, propName)}
	}

	var errs []string
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
	if propDef.Type == PropertyTypeEnum && len(propDef.Values) == 0 {
		errs = append(errs, fmt.Sprintf(
			"%s: property %q is type \"enum\" but has no 'values' list", schemaName, propName))
	}
	if propDef.Unique && !isStringValuedType(propDef.Type) {
		errs = append(errs, fmt.Sprintf(
			"%s: property %q has 'unique: true' on non-string type %q; "+
				"unique is only supported on string-valued properties",
			schemaName, propName, propDef.Type))
	}
	errs = append(errs, validateComputedProperty(schemaName, propName, propDef, allowComputed)...)
	errs = append(errs, validateFilePropertyOptions(schemaName, propName, propDef)...)
	return errs
}

func validateComputedProperty(schemaName, propName string, propDef PropertyDef, allowComputed bool) []string {
	if propDef.Computed == "" {
		return nil
	}
	var errs []string
	if !allowComputed {
		errs = append(errs, fmt.Sprintf("%s: property %q: computed is only supported on entity properties", schemaName, propName))
	}
	if propDef.List {
		errs = append(errs, fmt.Sprintf("%s: property %q: computed list properties are not supported", schemaName, propName))
	}
	if propDef.Type == PropertyTypeFile {
		errs = append(errs, fmt.Sprintf("%s: property %q: computed file properties are not supported", schemaName, propName))
	}
	if propDef.Default != "" {
		errs = append(errs, fmt.Sprintf("%s: property %q: computed and default are mutually exclusive", schemaName, propName))
	}
	return errs
}

// isStringValuedType reports whether a property of this type stores its value
// as a string (so the application uniqueness check, which reads values as
// strings, can see it). The built-in string-ish types qualify; integer,
// boolean, and file do not. Any non-built-in type name is a custom type
// (enum/regex, defined in `types:`), whose values are strings, so it qualifies.
func isStringValuedType(typeName string) bool {
	switch typeName {
	case PropertyTypeString, PropertyTypeDate, PropertyTypeDatetime, PropertyTypeEnum, PropertyTypeRrule:
		return true
	case PropertyTypeInteger, PropertyTypeBoolean, PropertyTypeFile:
		return false
	default:
		// A custom type (declared in `types:`) — enum or regex-validated string.
		return true
	}
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
		errs = append(errs, validateTransformSteps(schemaName, propName, propDef.Transform)...)
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

// validImageReencodeTargets is the allowlist of native re-encode output
// formats. Both are always within the default-safe MIME allowlist, so a native
// image step's output cannot escape the allowlist (RR-4G5YBU).
var validImageReencodeTargets = map[string]bool{"jpeg": true, "png": true}

// validateTransformSteps checks each transform step is well-formed: exactly one
// of cmd/image is set, and a native image step names a known re-encode target.
func validateTransformSteps(schemaName, propName string, steps []TransformStep) []string {
	var errs []string
	for i, step := range steps {
		switch step.Kind() {
		case "cmd":
			// Existing external-command step; validated at runtime, not here.
		case "image":
			if r := step.Image.Reencode; r != "" && !validImageReencodeTargets[r] {
				errs = append(errs, fmt.Sprintf(
					"%s: property %q transform step %d has unknown image reencode %q; want \"jpeg\" or \"png\"",
					schemaName, propName, i, r))
			}
			if q := step.Image.Quality; q != 0 && (q < 1 || q > 100) {
				errs = append(errs, fmt.Sprintf(
					"%s: property %q transform step %d has image quality %d; must be 1..100 (0 = default)",
					schemaName, propName, i, q))
			}
		default:
			errs = append(errs, fmt.Sprintf(
				"%s: property %q transform step %d must set exactly one of \"cmd\" or \"image\"",
				schemaName, propName, i))
		}
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
	var raw map[string]any
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

// validateValidationFaces checks that every face named in a rule's `faces:`
// scope is one the target type declares (TKT-4Y6CMV).
//
// A load error rather than a silent no-match, for the same reason an
// unparseable `when:` is: a rule scoped to a face that does not exist matches
// nothing, so it passes forever while appearing to guard something. That is
// the worst failure mode a validator has — a check that reports success over
// data it never looked at.
//
// A rule with no `entity_type:` applies to every type, and types legitimately
// declare different faces, so the scope is satisfied if ANY type declares the
// name. Only a name no type declares is rejected.
func validateValidationFaces(m *Metamodel) []string {
	var errs []string
	for _, rule := range m.Validations {
		if len(rule.Faces) == 0 {
			continue
		}
		for _, face := range rule.Faces {
			if rule.EntityType != "" {
				def, ok := m.Entities[rule.EntityType]
				if !ok {
					continue // the unknown-type error is reported elsewhere
				}
				if _, declared := def.Faces[face]; !declared {
					// A type with NO faces gets its own message. The general
					// one would end in "(declares: )", which reads like a bug
					// in the error rather than the actual situation: the key
					// does not apply to this type at all.
					if len(def.Faces) == 0 {
						errs = append(errs, fmt.Sprintf(
							"validation %q: `faces: [%s]` but entity %q declares no `faces:` — "+
								"a type with one content state needs no scope; drop the key, "+
								"or declare the states on the type",
							rule.Name, face, rule.EntityType))
						continue
					}
					errs = append(errs, fmt.Sprintf(
						"validation %q: `faces: [%s]` names no declared face of entity %q "+
							"(declares: %s) — the rule would match nothing and pass forever",
						rule.Name, face, rule.EntityType,
						strings.Join(sortedKeys(def.Faces), ", ")))
				}
				continue
			}
			if !anyTypeDeclaresFace(m, face) {
				errs = append(errs, fmt.Sprintf(
					"validation %q: `faces: [%s]` names a face no entity type declares — "+
						"the rule would match nothing and pass forever", rule.Name, face))
			}
		}
	}
	return errs
}

// validateAutomationFaces checks that every face named in an automation's
// `on.faces:` scope is one the triggering type declares (TKT-4Y6CMV).
//
// A load error rather than a silent no-match, matching the rest of the
// automation loader: an unparseable `when:` and an uncompilable `condition:`
// are both load errors, because dropping a constraint widens the automation and
// a scope that matches nothing narrows it to nothing. Either way the operator
// wrote something that does not mean what it says.
func validateAutomationFaces(m *Metamodel) []string {
	var errs []string
	for _, auto := range m.Automations {
		if len(auto.On.Faces) == 0 {
			continue
		}
		for _, face := range auto.On.Faces {
			// `entity:` may name several types; the scope is satisfied if any
			// of them declares the face, for the same reason a validation
			// rule's is.
			if len(auto.On.Entity) > 0 {
				if !anyNamedTypeDeclaresFace(m, []string(auto.On.Entity), face) {
					// Distinguish "wrong name" from "this type has no states",
					// which need different fixes: correct the spelling versus
					// drop the key.
					if !anyNamedTypeHasFaces(m, []string(auto.On.Entity)) {
						errs = append(errs, fmt.Sprintf(
							"automation %q: `on.faces: [%s]` but %s declares no `faces:` — "+
								"a type with one content state needs no scope; drop the key, "+
								"or declare the states on the type",
							auto.Name, face, strings.Join([]string(auto.On.Entity), ", ")))
						continue
					}
					errs = append(errs, fmt.Sprintf(
						"automation %q: `on.faces: [%s]` names no declared face of %s — "+
							"the trigger would never fire",
						auto.Name, face, strings.Join([]string(auto.On.Entity), ", ")))
				}
				continue
			}
			if !anyTypeDeclaresFace(m, face) {
				errs = append(errs, fmt.Sprintf(
					"automation %q: `on.faces: [%s]` names a face no entity type declares — "+
						"the trigger would never fire", auto.Name, face))
			}
		}
	}
	return errs
}

// anyNamedTypeDeclaresFace reports whether any of the named types declares face.
func anyNamedTypeDeclaresFace(m *Metamodel, types []string, face string) bool {
	for _, t := range types {
		if def, ok := m.Entities[t]; ok {
			if _, declared := def.Faces[face]; declared {
				return true
			}
		}
	}
	return false
}

// anyNamedTypeHasFaces reports whether any of the named types declares content
// states at all — the difference between a misspelled face and a scope on a
// type that has none.
func anyNamedTypeHasFaces(m *Metamodel, types []string) bool {
	for _, t := range types {
		if def, ok := m.Entities[t]; ok && len(def.Faces) > 0 {
			return true
		}
	}
	return false
}
