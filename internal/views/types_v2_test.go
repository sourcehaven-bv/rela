package views

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestQueryNode_RootFields(t *testing.T) {
	input := `
entry_type: document
param: doc_id
`
	var node QueryNode
	err := yaml.Unmarshal([]byte(input), &node)
	require.NoError(t, err)

	assert.Equal(t, "document", node.EntryType)
	assert.Equal(t, "doc_id", node.Param)
	assert.True(t, node.IsRoot())
}

func TestQueryNode_ViaOutgoing(t *testing.T) {
	input := `via: describesBouwblok`
	var node QueryNode
	err := yaml.Unmarshal([]byte(input), &node)
	require.NoError(t, err)

	assert.Equal(t, "describesBouwblok", node.Via)
	assert.Empty(t, node.ViaIncoming)
}

func TestQueryNode_ViaIncoming(t *testing.T) {
	input := `via_incoming: partOfBouwblok`
	var node QueryNode
	err := yaml.Unmarshal([]byte(input), &node)
	require.NoError(t, err)

	assert.Empty(t, node.Via)
	assert.Equal(t, "partOfBouwblok", node.ViaIncoming)
}

func TestQueryNode_TypesFilterSingle(t *testing.T) {
	input := `types: [function]`
	var node QueryNode
	err := yaml.Unmarshal([]byte(input), &node)
	require.NoError(t, err)

	assert.Equal(t, []string{"function"}, node.Types)
}

func TestQueryNode_TypesFilterMultiple(t *testing.T) {
	input := `types: [function, usecase, scenario]`
	var node QueryNode
	err := yaml.Unmarshal([]byte(input), &node)
	require.NoError(t, err)

	assert.Equal(t, []string{"function", "usecase", "scenario"}, node.Types)
}

func TestQueryNode_Recursive(t *testing.T) {
	input := `recursive: 5`
	var node QueryNode
	err := yaml.Unmarshal([]byte(input), &node)
	require.NoError(t, err)

	assert.Equal(t, 5, node.Recursive)
	assert.True(t, node.IsRecursive())
}

func TestQueryNode_NotRecursive(t *testing.T) {
	input := `via: dependsOn`
	var node QueryNode
	err := yaml.Unmarshal([]byte(input), &node)
	require.NoError(t, err)

	assert.Equal(t, 0, node.Recursive)
	assert.False(t, node.IsRecursive())
}

func TestQueryNode_WhereClause(t *testing.T) {
	input := `where: "status=active"`
	var node QueryNode
	err := yaml.Unmarshal([]byte(input), &node)
	require.NoError(t, err)

	assert.Equal(t, "status=active", node.Where)
}

func TestQueryNode_RequireWithJSONPath(t *testing.T) {
	input := `
require:
  partOfBouwblok: "$.bouwbloks[*].id"
`
	var node QueryNode
	err := yaml.Unmarshal([]byte(input), &node)
	require.NoError(t, err)

	require.NotNil(t, node.Require)
	assert.Equal(t, "$.bouwbloks[*].id", node.Require["partOfBouwblok"])
}

func TestQueryNode_RequireMultipleRelations(t *testing.T) {
	input := `
require:
  partOfBouwblok: "$.bouwbloks[*].id"
  partOfSystem: "$.systems[*].id"
`
	var node QueryNode
	err := yaml.Unmarshal([]byte(input), &node)
	require.NoError(t, err)

	require.NotNil(t, node.Require)
	assert.Len(t, node.Require, 2)
	assert.Equal(t, "$.bouwbloks[*].id", node.Require["partOfBouwblok"])
	assert.Equal(t, "$.systems[*].id", node.Require["partOfSystem"])
}

func TestQueryNode_OnlyProperties(t *testing.T) {
	input := `only: [id, title, status]`
	var node QueryNode
	err := yaml.Unmarshal([]byte(input), &node)
	require.NoError(t, err)

	assert.Equal(t, []string{"id", "title", "status"}, node.Only)
}

func TestQueryNode_ContentFalse(t *testing.T) {
	input := `content: false`
	var node QueryNode
	err := yaml.Unmarshal([]byte(input), &node)
	require.NoError(t, err)

	require.NotNil(t, node.Content)
	assert.False(t, *node.Content)
	assert.False(t, node.IncludeContent())
}

func TestQueryNode_ContentTrueExplicit(t *testing.T) {
	input := `content: true`
	var node QueryNode
	err := yaml.Unmarshal([]byte(input), &node)
	require.NoError(t, err)

	require.NotNil(t, node.Content)
	assert.True(t, *node.Content)
	assert.True(t, node.IncludeContent())
}

func TestQueryNode_ContentDefault(t *testing.T) {
	input := `via: test`
	var node QueryNode
	err := yaml.Unmarshal([]byte(input), &node)
	require.NoError(t, err)

	assert.Nil(t, node.Content)
	assert.True(t, node.IncludeContent()) // Default is true
}

func TestQueryNode_PropsFalse(t *testing.T) {
	input := `props: false`
	var node QueryNode
	err := yaml.Unmarshal([]byte(input), &node)
	require.NoError(t, err)

	require.NotNil(t, node.Props)
	assert.False(t, *node.Props)
	assert.False(t, node.IncludeProps())
}

func TestQueryNode_PropsDefault(t *testing.T) {
	input := `via: test`
	var node QueryNode
	err := yaml.Unmarshal([]byte(input), &node)
	require.NoError(t, err)

	assert.Nil(t, node.Props)
	assert.True(t, node.IncludeProps()) // Default is true
}

func TestQueryNode_NestedRelations(t *testing.T) {
	input := `
entry_type: document
param: doc_id

relations:
  bouwbloks:
    via: describesBouwblok

    relations:
      functions:
        via_incoming: partOfBouwblok
        types: [function]

        relations:
          components:
            via_incoming: realizes
            types: [component]
            recursive: 5
            require:
              partOfBouwblok: "$.bouwbloks[*].id"
`
	var node QueryNode
	err := yaml.Unmarshal([]byte(input), &node)
	require.NoError(t, err)

	// Root level
	assert.Equal(t, "document", node.EntryType)
	assert.Equal(t, "doc_id", node.Param)
	assert.True(t, node.HasChildren())

	// Level 1: bouwbloks
	bouwbloks, ok := node.Relations["bouwbloks"]
	require.True(t, ok)
	assert.Equal(t, "describesBouwblok", bouwbloks.Via)
	assert.True(t, bouwbloks.HasChildren())

	// Level 2: functions
	functions, ok := bouwbloks.Relations["functions"]
	require.True(t, ok)
	assert.Equal(t, "partOfBouwblok", functions.ViaIncoming)
	assert.Equal(t, []string{"function"}, functions.Types)
	assert.True(t, functions.HasChildren())

	// Level 3: components
	components, ok := functions.Relations["components"]
	require.True(t, ok)
	assert.Equal(t, "realizes", components.ViaIncoming)
	assert.Equal(t, []string{"component"}, components.Types)
	assert.Equal(t, 5, components.Recursive)
	assert.Equal(t, "$.bouwbloks[*].id", components.Require["partOfBouwblok"])
	assert.False(t, components.HasChildren()) // Leaf node
}

func TestQueryNode_LeafNode(t *testing.T) {
	input := `via: describesBouwblok`
	var node QueryNode
	err := yaml.Unmarshal([]byte(input), &node)
	require.NoError(t, err)

	assert.False(t, node.HasChildren())
	assert.False(t, node.IsRoot())
}

func TestViewDefV2_Unmarshal(t *testing.T) {
	input := `
description: "Complete context for document publishing"
entry_type: document
param: doc_id

relations:
  bouwbloks:
    via: describesBouwblok
`
	var view ViewDefV2
	err := yaml.Unmarshal([]byte(input), &view)
	require.NoError(t, err)

	assert.Equal(t, "Complete context for document publishing", view.Description)
	assert.Equal(t, "document", view.EntryType)
	assert.Equal(t, "doc_id", view.Param)
	require.NotNil(t, view.Relations)
	assert.Contains(t, view.Relations, "bouwbloks")
}

func TestFileV2_Unmarshal(t *testing.T) {
	input := `
views:
  document_publish:
    description: "Publish documents"
    entry_type: document
    param: doc_id
    relations:
      sections:
        via: contains

  simple_view:
    entry_type: component
    param: comp_id
`
	var file FileV2
	err := yaml.Unmarshal([]byte(input), &file)
	require.NoError(t, err)

	assert.Len(t, file.Views, 2)

	// Check document_publish
	docView, ok := file.GetView("document_publish")
	require.True(t, ok)
	assert.Equal(t, "Publish documents", docView.Description)
	assert.Equal(t, "document", docView.EntryType)

	// Check simple_view
	simpleView, ok := file.GetView("simple_view")
	require.True(t, ok)
	assert.Equal(t, "component", simpleView.EntryType)

	// Check ViewNames
	names := file.ViewNames()
	assert.Len(t, names, 2)
	assert.Contains(t, names, "document_publish")
	assert.Contains(t, names, "simple_view")
}

func TestFileV2_GetView_NotFound(t *testing.T) {
	file := &FileV2{Views: map[string]*ViewDefV2{}}
	view, ok := file.GetView("nonexistent")
	assert.False(t, ok)
	assert.Nil(t, view)
}

func TestFileV2_Empty(t *testing.T) {
	var file FileV2
	err := yaml.Unmarshal([]byte("{}"), &file)
	require.NoError(t, err)

	view, ok := file.GetView("anything")
	assert.False(t, ok)
	assert.Nil(t, view)

	names := file.ViewNames()
	assert.Nil(t, names)
}

func TestQueryNode_ComplexExample(t *testing.T) {
	// This tests a realistic, complex view definition
	input := `
description: "GF Adressering document context"
entry_type: document
param: doc_id

relations:
  scopes:
    via: definesScope
    only: [id, title, scope_type]

  bouwbloks:
    via: describesBouwblok

    relations:
      functions:
        via_incoming: partOfBouwblok
        types: [function]

        relations:
          components:
            via_incoming: realizes
            types: [component]

          usecases:
            via_incoming: demonstratesFunction
            types: [usecase]

      components:
        via_incoming: partOfBouwblok
        types: [component]

        relations:
          dependencies:
            via: dependsOn
            types: [component]
            recursive: 5
            require:
              partOfBouwblok: "$.bouwbloks[*].id"

      personas:
        via_incoming: partOfBouwblok
        types: [persona]
        content: false

        relations:
          uses_functions:
            via: usesFunction
            types: [function]
            only: [id, title]
            props: false

  systems:
    via: describesSystem
    where: "status=active"
`
	var view ViewDefV2
	err := yaml.Unmarshal([]byte(input), &view)
	require.NoError(t, err)

	// Verify structure
	assert.Equal(t, "GF Adressering document context", view.Description)
	assert.Equal(t, "document", view.EntryType)

	// Check scopes
	scopes := view.Relations["scopes"]
	require.NotNil(t, scopes)
	assert.Equal(t, "definesScope", scopes.Via)
	assert.Equal(t, []string{"id", "title", "scope_type"}, scopes.Only)

	// Check bouwbloks -> functions -> usecases path
	bouwbloks := view.Relations["bouwbloks"]
	require.NotNil(t, bouwbloks)
	functions := bouwbloks.Relations["functions"]
	require.NotNil(t, functions)
	usecases := functions.Relations["usecases"]
	require.NotNil(t, usecases)
	assert.Equal(t, "demonstratesFunction", usecases.ViaIncoming)

	// Check personas with content: false
	personas := bouwbloks.Relations["personas"]
	require.NotNil(t, personas)
	assert.False(t, personas.IncludeContent())

	// Check uses_functions with props: false
	usesFunctions := personas.Relations["uses_functions"]
	require.NotNil(t, usesFunctions)
	assert.False(t, usesFunctions.IncludeProps())

	// Check systems with where clause
	systems := view.Relations["systems"]
	require.NotNil(t, systems)
	assert.Equal(t, "status=active", systems.Where)
}

func TestIsV2Format(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name: "v2 format with entry_type",
			input: `
views:
  test:
    entry_type: document
    param: doc_id
`,
			expected: true,
		},
		{
			name: "v1 format with entry.type",
			input: `
views:
  test:
    entry:
      type: document
      parameter: doc_id
`,
			expected: false,
		},
		{
			name:     "empty views",
			input:    `views: {}`,
			expected: false,
		},
		{
			name:     "invalid yaml",
			input:    `{{{`,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsV2Format([]byte(tt.input))
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseV2(t *testing.T) {
	input := `
views:
  test_view:
    entry_type: document
    param: doc_id
    relations:
      items:
        via: contains
`
	file, err := ParseV2([]byte(input))
	require.NoError(t, err)

	view, ok := file.GetView("test_view")
	require.True(t, ok)
	assert.Equal(t, "document", view.EntryType)
	assert.Equal(t, "doc_id", view.Param)
	require.NotNil(t, view.Relations["items"])
}

func TestParseV2_Empty(t *testing.T) {
	input := `{}`
	file, err := ParseV2([]byte(input))
	require.NoError(t, err)
	assert.NotNil(t, file.Views)
	assert.Empty(t, file.Views)
}

func TestParseV2_InvalidYAML(t *testing.T) {
	input := `{{{invalid`
	_, err := ParseV2([]byte(input))
	assert.Error(t, err)
}
