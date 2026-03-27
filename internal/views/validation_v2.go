package views

import (
	"fmt"
	"sort"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// Validate checks if a v2 views file is valid against a metamodel.
func (f *FileV2) Validate(meta *metamodel.Metamodel) error {
	for viewName, view := range f.Views {
		if err := view.Validate(meta, viewName); err != nil {
			return err
		}
	}
	return nil
}

// Validate checks if a v2 view definition is valid.
func (v *ViewDefV2) Validate(meta *metamodel.Metamodel, viewName string) error {
	// Root must have entry_type
	if v.EntryType == "" {
		return &ValidationError{
			View:    viewName,
			Field:   "entry_type",
			Message: "root view must specify entry_type",
		}
	}

	// Validate entry type exists in metamodel
	if _, ok := meta.GetEntityDef(v.EntryType); !ok {
		return &ValidationError{
			View:    viewName,
			Field:   "entry_type",
			Message: fmt.Sprintf("unknown entity type: %s", v.EntryType),
		}
	}

	// Root must have param
	if v.Param == "" {
		return &ValidationError{
			View:    viewName,
			Field:   "param",
			Message: "root view must specify param",
		}
	}

	// Root should not have via or via_incoming (entry point has no traversal)
	if v.Via != "" {
		return &ValidationError{
			View:    viewName,
			Field:   "via",
			Message: "root view cannot have via (use entry_type instead)",
		}
	}
	if v.ViaIncoming != "" {
		return &ValidationError{
			View:    viewName,
			Field:   "via_incoming",
			Message: "root view cannot have via_incoming (use entry_type instead)",
		}
	}

	// Validate child relations
	for childName, child := range v.Relations {
		if err := child.validateAsChild(meta, viewName, childName); err != nil {
			return err
		}
	}

	return nil
}

// validateAsChild validates a QueryNode that is a child (non-root) node.
func (q *QueryNode) validateAsChild(meta *metamodel.Metamodel, viewName, path string) error {
	// Child nodes must not have entry_type or param (root-only fields)
	if q.EntryType != "" {
		return &ValidationError{
			View:    viewName,
			Field:   fmt.Sprintf("%s.entry_type", path),
			Message: "entry_type is only valid at root level",
		}
	}
	if q.Param != "" {
		return &ValidationError{
			View:    viewName,
			Field:   fmt.Sprintf("%s.param", path),
			Message: "param is only valid at root level",
		}
	}

	// Child nodes must have via or via_incoming (defines how to traverse)
	if q.Via == "" && q.ViaIncoming == "" {
		return &ValidationError{
			View:    viewName,
			Field:   path,
			Message: "must specify either 'via' or 'via_incoming'",
		}
	}

	// Cannot have both via and via_incoming
	if q.Via != "" && q.ViaIncoming != "" {
		return &ValidationError{
			View:    viewName,
			Field:   path,
			Message: "cannot specify both 'via' and 'via_incoming'",
		}
	}

	// Validate relation type exists
	relationType := q.Via
	if relationType == "" {
		relationType = q.ViaIncoming
	}
	if _, ok := meta.GetRelationDef(relationType); !ok {
		return &ValidationError{
			View:    viewName,
			Field:   fmt.Sprintf("%s.via", path),
			Message: fmt.Sprintf("unknown relation type: %s", relationType),
		}
	}

	// Validate types filter - each type must exist in metamodel
	for _, typeName := range q.Types {
		if _, ok := meta.GetEntityDef(typeName); !ok {
			return &ValidationError{
				View:    viewName,
				Field:   fmt.Sprintf("%s.types", path),
				Message: fmt.Sprintf("unknown entity type: %s", typeName),
			}
		}
	}

	// Validate recursive depth is non-negative
	if q.Recursive < 0 {
		return &ValidationError{
			View:    viewName,
			Field:   fmt.Sprintf("%s.recursive", path),
			Message: "recursive depth must be non-negative",
		}
	}

	// Validate require relations exist
	for reqRelation := range q.Require {
		if _, ok := meta.GetRelationDef(reqRelation); !ok {
			return &ValidationError{
				View:    viewName,
				Field:   fmt.Sprintf("%s.require.%s", path, reqRelation),
				Message: fmt.Sprintf("unknown relation type: %s", reqRelation),
			}
		}
	}

	// Recursively validate child relations
	for childName, child := range q.Relations {
		childPath := fmt.Sprintf("%s.relations.%s", path, childName)
		if err := child.validateAsChild(meta, viewName, childPath); err != nil {
			return err
		}
	}

	return nil
}

// SortedViewNames returns all view names in sorted order.
// This provides deterministic ordering for consistent output.
func (f *FileV2) SortedViewNames() []string {
	if f.Views == nil {
		return nil
	}
	names := make([]string, 0, len(f.Views))
	for name := range f.Views {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
