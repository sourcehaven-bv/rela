package views

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// testMetamodel creates a minimal metamodel for validation tests.
func testMetamodel() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"document":  {Properties: map[string]metamodel.PropertyDef{}},
			"component": {Properties: map[string]metamodel.PropertyDef{}},
			"function":  {Properties: map[string]metamodel.PropertyDef{}},
		},
		Relations: map[string]metamodel.RelationDef{
			"describes":      {},
			"partOfBouwblok": {},
			"realizes":       {},
			"dependsOn":      {},
		},
	}
}

func TestViewDefV2_Validate_Valid(t *testing.T) {
	meta := testMetamodel()
	view := &ViewDefV2{
		QueryNode: QueryNode{
			EntryType: "document",
			Param:     "doc_id",
			Relations: map[string]*QueryNode{
				"components": {
					Via:   "describes",
					Types: []string{"component"},
				},
			},
		},
		Description: "Test view",
	}

	err := view.Validate(meta, "test_view")
	assert.NoError(t, err)
}

func TestViewDefV2_Validate_MissingEntryType(t *testing.T) {
	meta := testMetamodel()
	view := &ViewDefV2{
		QueryNode: QueryNode{
			Param: "doc_id",
		},
	}

	err := view.Validate(meta, "test_view")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "entry_type")
	assert.Contains(t, err.Error(), "root view must specify entry_type")
}

func TestViewDefV2_Validate_MissingParam(t *testing.T) {
	meta := testMetamodel()
	view := &ViewDefV2{
		QueryNode: QueryNode{
			EntryType: "document",
		},
	}

	err := view.Validate(meta, "test_view")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "param")
	assert.Contains(t, err.Error(), "root view must specify param")
}

func TestViewDefV2_Validate_UnknownEntryType(t *testing.T) {
	meta := testMetamodel()
	view := &ViewDefV2{
		QueryNode: QueryNode{
			EntryType: "unknown_type",
			Param:     "doc_id",
		},
	}

	err := view.Validate(meta, "test_view")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown entity type: unknown_type")
}

func TestViewDefV2_Validate_RootCannotHaveVia(t *testing.T) {
	meta := testMetamodel()
	view := &ViewDefV2{
		QueryNode: QueryNode{
			EntryType: "document",
			Param:     "doc_id",
			Via:       "describes",
		},
	}

	err := view.Validate(meta, "test_view")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "root view cannot have via")
}

func TestViewDefV2_Validate_RootCannotHaveViaIncoming(t *testing.T) {
	meta := testMetamodel()
	view := &ViewDefV2{
		QueryNode: QueryNode{
			EntryType:   "document",
			Param:       "doc_id",
			ViaIncoming: "describes",
		},
	}

	err := view.Validate(meta, "test_view")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "root view cannot have via_incoming")
}

func TestQueryNode_ValidateAsChild_CannotHaveEntryType(t *testing.T) {
	meta := testMetamodel()
	view := &ViewDefV2{
		QueryNode: QueryNode{
			EntryType: "document",
			Param:     "doc_id",
			Relations: map[string]*QueryNode{
				"components": {
					Via:       "describes",
					EntryType: "component", // Invalid: entry_type on child
				},
			},
		},
	}

	err := view.Validate(meta, "test_view")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "entry_type is only valid at root level")
}

func TestQueryNode_ValidateAsChild_CannotHaveParam(t *testing.T) {
	meta := testMetamodel()
	view := &ViewDefV2{
		QueryNode: QueryNode{
			EntryType: "document",
			Param:     "doc_id",
			Relations: map[string]*QueryNode{
				"components": {
					Via:   "describes",
					Param: "comp_id", // Invalid: param on child
				},
			},
		},
	}

	err := view.Validate(meta, "test_view")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "param is only valid at root level")
}

func TestQueryNode_ValidateAsChild_MustHaveViaOrViaIncoming(t *testing.T) {
	meta := testMetamodel()
	view := &ViewDefV2{
		QueryNode: QueryNode{
			EntryType: "document",
			Param:     "doc_id",
			Relations: map[string]*QueryNode{
				"components": {
					Types: []string{"component"}, // Missing via or via_incoming
				},
			},
		},
	}

	err := view.Validate(meta, "test_view")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must specify either 'via' or 'via_incoming'")
}

func TestQueryNode_ValidateAsChild_CannotHaveBothViaAndViaIncoming(t *testing.T) {
	meta := testMetamodel()
	view := &ViewDefV2{
		QueryNode: QueryNode{
			EntryType: "document",
			Param:     "doc_id",
			Relations: map[string]*QueryNode{
				"components": {
					Via:         "describes",
					ViaIncoming: "partOfBouwblok",
				},
			},
		},
	}

	err := view.Validate(meta, "test_view")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot specify both 'via' and 'via_incoming'")
}

func TestQueryNode_ValidateAsChild_UnknownRelationType(t *testing.T) {
	meta := testMetamodel()
	view := &ViewDefV2{
		QueryNode: QueryNode{
			EntryType: "document",
			Param:     "doc_id",
			Relations: map[string]*QueryNode{
				"components": {
					Via: "unknownRelation",
				},
			},
		},
	}

	err := view.Validate(meta, "test_view")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown relation type: unknownRelation")
}

func TestQueryNode_ValidateAsChild_UnknownTypeFilter(t *testing.T) {
	meta := testMetamodel()
	view := &ViewDefV2{
		QueryNode: QueryNode{
			EntryType: "document",
			Param:     "doc_id",
			Relations: map[string]*QueryNode{
				"components": {
					Via:   "describes",
					Types: []string{"unknownType"},
				},
			},
		},
	}

	err := view.Validate(meta, "test_view")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown entity type: unknownType")
}

func TestQueryNode_ValidateAsChild_UnknownRequireRelation(t *testing.T) {
	meta := testMetamodel()
	view := &ViewDefV2{
		QueryNode: QueryNode{
			EntryType: "document",
			Param:     "doc_id",
			Relations: map[string]*QueryNode{
				"components": {
					Via: "describes",
					Require: map[string]string{
						"unknownRelation": "$.root[*].id",
					},
				},
			},
		},
	}

	err := view.Validate(meta, "test_view")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown relation type: unknownRelation")
}

func TestQueryNode_ValidateAsChild_NestedValidation(t *testing.T) {
	meta := testMetamodel()
	view := &ViewDefV2{
		QueryNode: QueryNode{
			EntryType: "document",
			Param:     "doc_id",
			Relations: map[string]*QueryNode{
				"components": {
					Via: "describes",
					Relations: map[string]*QueryNode{
						"functions": {
							ViaIncoming: "realizes",
							Relations: map[string]*QueryNode{
								"deps": {
									// Missing via/via_incoming at nested level
									Types: []string{"component"},
								},
							},
						},
					},
				},
			},
		},
	}

	err := view.Validate(meta, "test_view")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "components.relations.functions.relations.deps")
	assert.Contains(t, err.Error(), "must specify either 'via' or 'via_incoming'")
}

func TestQueryNode_ValidateAsChild_ViaIncoming(t *testing.T) {
	meta := testMetamodel()
	view := &ViewDefV2{
		QueryNode: QueryNode{
			EntryType: "document",
			Param:     "doc_id",
			Relations: map[string]*QueryNode{
				"functions": {
					ViaIncoming: "partOfBouwblok",
					Types:       []string{"function"},
				},
			},
		},
	}

	err := view.Validate(meta, "test_view")
	assert.NoError(t, err)
}

func TestFileV2_Validate_AllViews(t *testing.T) {
	meta := testMetamodel()
	file := &FileV2{
		Views: map[string]*ViewDefV2{
			"view1": {
				QueryNode: QueryNode{
					EntryType: "document",
					Param:     "doc_id",
				},
			},
			"view2": {
				QueryNode: QueryNode{
					EntryType: "component",
					Param:     "comp_id",
				},
			},
		},
	}

	err := file.Validate(meta)
	assert.NoError(t, err)
}

func TestFileV2_Validate_FailsOnFirstInvalid(t *testing.T) {
	meta := testMetamodel()
	file := &FileV2{
		Views: map[string]*ViewDefV2{
			"valid_view": {
				QueryNode: QueryNode{
					EntryType: "document",
					Param:     "doc_id",
				},
			},
			"invalid_view": {
				QueryNode: QueryNode{
					// Missing entry_type and param
				},
			},
		},
	}

	err := file.Validate(meta)
	require.Error(t, err)
	// Should fail on one of the views
	assert.Contains(t, err.Error(), "entry_type")
}

func TestFileV2_SortedViewNames(t *testing.T) {
	file := &FileV2{
		Views: map[string]*ViewDefV2{
			"zebra": {},
			"alpha": {},
			"mango": {},
		},
	}

	names := file.SortedViewNames()
	assert.Equal(t, []string{"alpha", "mango", "zebra"}, names)
}

func TestFileV2_SortedViewNames_Empty(t *testing.T) {
	file := &FileV2{Views: map[string]*ViewDefV2{}}
	names := file.SortedViewNames()
	assert.Empty(t, names)
}

func TestFileV2_SortedViewNames_Nil(t *testing.T) {
	file := &FileV2{}
	names := file.SortedViewNames()
	assert.Nil(t, names)
}

func TestQueryNode_HasChildren_NilRelations(t *testing.T) {
	node := &QueryNode{
		Via:       "describes",
		Relations: nil,
	}
	assert.False(t, node.HasChildren())
}

func TestQueryNode_HasChildren_EmptyRelations(t *testing.T) {
	node := &QueryNode{
		Via:       "describes",
		Relations: map[string]*QueryNode{},
	}
	assert.False(t, node.HasChildren())
}

func TestQueryNode_HasChildren_WithRelations(t *testing.T) {
	node := &QueryNode{
		Via: "describes",
		Relations: map[string]*QueryNode{
			"children": {Via: "contains"},
		},
	}
	assert.True(t, node.HasChildren())
}
