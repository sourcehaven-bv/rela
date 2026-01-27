package metamodel

import (
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Load reads and parses a metamodel from a YAML file
func Load(path string) (*Metamodel, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return Parse(data)
}

// Parse parses metamodel YAML content
func Parse(data []byte) (*Metamodel, error) {
	var m Metamodel
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, err
	}

	// Build alias map and validate entity definitions
	m.aliasMap = make(map[string]string)
	for name, def := range m.Entities {
		// Validate id_type if specified
		if def.IDType != "" && def.IDType != IDTypeSequential && def.IDType != IDTypeString {
			return nil, &InvalidIDTypeError{EntityType: name, IDType: def.IDType}
		}

		// Validate property names
		for propName := range def.Properties {
			// Reject property names with leading or trailing whitespace
			// This prevents bypassing reserved name checks with " id" or "type " etc.
			trimmedName := strings.TrimSpace(propName)
			if trimmedName != propName {
				return nil, &WhitespacePropertyError{EntityType: name, PropertyName: propName}
			}

			// Check for reserved property names
			if ReservedPropertyNames[propName] {
				return nil, &ReservedPropertyError{EntityType: name, PropertyName: propName}
			}
		}

		// Add lowercase name as self-reference
		m.aliasMap[strings.ToLower(name)] = name
		// Add all aliases
		for _, alias := range def.Aliases {
			m.aliasMap[strings.ToLower(alias)] = name
		}
	}

	return &m, nil
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
				Label:      "Requirement",
				Aliases:    []string{"req"},
				IDPatterns: []string{"REQ-"},
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
				IDPatterns: []string{"DEC-", "ADR-"},
				Properties: map[string]PropertyDef{
					"title":     {Type: "string", Required: true},
					"rationale": {Type: "string"},
					"status":    {Type: "status", Required: true},
				},
			},
			"solution": {
				Label:      "Solution",
				Aliases:    []string{"sol"},
				IDPatterns: []string{"SOL-"},
				Properties: map[string]PropertyDef{
					"title":       {Type: "string", Required: true},
					"description": {Type: "string"},
					"status":      {Type: "status"},
				},
			},
			"component": {
				Label:      "Component",
				Aliases:    []string{"comp"},
				IDPatterns: []string{"COMP-", "AC-", "TC-"},
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
				Inverse:     &InverseDef{Name: "addressedBy", Label: "addressed by"},
			},
			"implements": {
				Label:       "implements",
				Description: "A solution implements a decision",
				From:        []string{"solution"},
				To:          []string{"decision"},
				Inverse:     &InverseDef{Name: "implementedBy", Label: "implemented by"},
			},
			"realizes": {
				Label:       "realizes",
				Description: "A component realizes a solution",
				From:        []string{"component"},
				To:          []string{"solution"},
				Inverse:     &InverseDef{Name: "realizedBy", Label: "realized by"},
			},
			"dependsOn": {
				Label:   "depends on",
				From:    []string{"component", "solution", "decision"},
				To:      []string{"component", "solution", "decision"},
				Inverse: &InverseDef{Name: "dependencyOf", Label: "dependency of"},
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
    id_patterns: ["REQ-"]
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
    id_patterns: ["DEC-", "ADR-"]
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
    id_patterns: ["SOL-"]
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
    id_patterns: ["COMP-", "AC-", "TC-"]
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
    inverse:
      name: addressedBy
      label: addressed by

  implements:
    label: implements
    description: A solution implements a decision
    from: [solution]
    to: [decision]
    inverse:
      name: implementedBy
      label: implemented by

  realizes:
    label: realizes
    description: A component realizes a solution
    from: [component]
    to: [solution]
    inverse:
      name: realizedBy
      label: realized by

  dependsOn:
    label: depends on
    from: [component, solution, decision]
    to: [component, solution, decision]
    inverse:
      name: dependencyOf
      label: dependency of

# Custom validation rules (optional)
# Define rules to check entity properties using filter expressions.
# Uses the same syntax as --where filters: =, !=, <, <=, >, >=, =~ (regex)
#
# validations:
#   - name: accepted-requirements-need-priority
#     description: "Accepted requirements must have a priority assigned"
#     entity_type: requirement
#     match:
#       - "status=accepted"
#     require:
#       - "priority!="
#     severity: error
#
#   - name: decisions-need-rationale
#     description: "All decisions should have a rationale"
#     entity_type: decision
#     require:
#       - "rationale!="
#     severity: warning
`
}
