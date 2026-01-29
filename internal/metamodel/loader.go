package metamodel

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/migration"
)

// validTopLevelKeys are the recognized top-level keys in a metamodel YAML file.
var validTopLevelKeys = map[string]bool{
	"version":     true,
	"namespace":   true,
	"types":       true,
	"entities":    true,
	"relations":   true,
	"validations": true,
	"includes":    true,
}

// knownTypos maps common misspellings to the correct key name.
var knownTypos = map[string]string{
	"entity":     "entities",
	"type":       "types",
	"relation":   "relations",
	"validation": "validations",
}

// Load reads and parses a metamodel from a YAML file.
// If the metamodel contains an `includes:` key, included files are recursively
// loaded and merged. Include paths are resolved relative to the directory
// containing the metamodel file.
// Returns a MigrationError if the file contains deprecated syntax that needs migration.
func Load(path string) (*Metamodel, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Check for deprecated syntax that needs migration
	detections, err := migration.Detect(path, migration.FileTypeMetamodel)
	if err != nil {
		return nil, err
	}
	if len(detections) > 0 {
		return nil, &migration.Error{
			FilePath:   path,
			Detections: detections,
		}
	}

	// When includes are present, parse without full validation first,
	// resolve includes, then validate the merged result.
	m, err := parseRaw(data)
	if err != nil {
		return nil, err
	}

	if len(m.Includes) > 0 {
		rootDir := filepath.Dir(path)
		if err := loadWithIncludes(m, path, rootDir); err != nil {
			return nil, err
		}
		// Validate the fully merged metamodel
		if err := validate(m); err != nil {
			return nil, err
		}
		return m, nil
	}

	// No includes: validate immediately
	if err := validate(m); err != nil {
		return nil, err
	}

	return m, nil
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

	return &m, nil
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

	validationErrors = append(validationErrors, validateEntitySemantics(m)...)
	validationErrors = append(validationErrors, validateRelationReferences(m)...)

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
		if def.IDType != "" && def.IDType != IDTypeAuto && def.IDType != IDTypeManual {
			return &InvalidIDTypeError{EntityType: name, IDType: def.IDType}
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
		if def.GetIDType() == IDTypeAuto && def.IDPrefix == "" && len(def.IDPrefixes) == 0 {
			errs = append(errs, fmt.Sprintf(
				"entity %q: no ID prefix defined (set 'id_prefix' or 'id_prefixes', or use 'id_type: manual')", name))
		}

		for propName, propDef := range def.Properties {
			if propDef.Type == "" {
				errs = append(errs, fmt.Sprintf("entity %q: property %q has no type specified", name, propName))
				continue
			}
			if !isKnownPropertyType(propDef.Type, m) {
				errs = append(errs, fmt.Sprintf(
					"entity %q: property %q has unknown type %q (not a built-in type and not defined in 'types')",
					name, propName, propDef.Type))
			}
			if propDef.Type == PropertyTypeEnum && len(propDef.Values) == 0 {
				errs = append(errs, fmt.Sprintf(
					"entity %q: property %q is type \"enum\" but has no 'values' list", name, propName))
			}
		}
	}

	return errs
}

// validateRelationReferences checks that all entity types referenced in relations exist.
func validateRelationReferences(m *Metamodel) []string {
	var errs []string

	relNames := sortedKeys(m.Relations)
	for _, name := range relNames {
		rel := m.Relations[name]
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
