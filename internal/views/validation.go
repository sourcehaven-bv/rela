package views

import (
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// Validate checks if a views file is valid against a metamodel
func (vf *File) Validate(meta *metamodel.Metamodel) error {
	for viewName, view := range vf.Views {
		if err := view.Validate(meta, viewName); err != nil {
			return err
		}
	}
	return nil
}

// Validate checks if a view definition is valid
func (v *ViewDef) Validate(meta *metamodel.Metamodel, viewName string) error {
	// Validate entry type exists
	if _, ok := meta.GetEntityDef(v.Entry.Type); !ok {
		return &ValidationError{
			View:    viewName,
			Field:   "entry.type",
			Message: fmt.Sprintf("unknown entity type: %s", v.Entry.Type),
		}
	}

	// Validate traverse rules
	for i, rule := range v.Traverse {
		if err := rule.Validate(meta, viewName, i); err != nil {
			return err
		}
	}

	// Validate derived collections reference valid source collections
	for derivedName, derived := range v.Derived {
		if err := derived.Validate(viewName, derivedName); err != nil {
			return err
		}
	}

	// Validate relation exports
	for i, export := range v.RelationExports {
		if err := export.Validate(meta, viewName, i); err != nil {
			return err
		}
	}

	return nil
}

// Validate checks if a traverse rule is valid
func (t *TraverseRule) Validate(meta *metamodel.Metamodel, viewName string, index int) error {
	// Must have either follow or follow_incoming
	if t.Follow == "" && t.FollowIncoming == "" {
		return &ValidationError{
			View:    viewName,
			Field:   fmt.Sprintf("traverse[%d]", index),
			Message: "must specify either 'follow' or 'follow_incoming'",
		}
	}

	// Cannot have both follow and follow_incoming
	if t.Follow != "" && t.FollowIncoming != "" {
		return &ValidationError{
			View:    viewName,
			Field:   fmt.Sprintf("traverse[%d]", index),
			Message: "cannot specify both 'follow' and 'follow_incoming'",
		}
	}

	// Validate relation type exists
	relationType := t.Follow
	if relationType == "" {
		relationType = t.FollowIncoming
	}
	if _, ok := meta.GetRelationDef(relationType); !ok {
		return &ValidationError{
			View:    viewName,
			Field:   fmt.Sprintf("traverse[%d]", index),
			Message: fmt.Sprintf("unknown relation type: %s", relationType),
		}
	}

	// Must have collect_as
	if t.CollectAs == nil {
		return &ValidationError{
			View:    viewName,
			Field:   fmt.Sprintf("traverse[%d]", index),
			Message: "must specify 'collect_as'",
		}
	}

	// Validate max_depth
	if t.Recursive && t.MaxDepth < 0 {
		return &ValidationError{
			View:    viewName,
			Field:   fmt.Sprintf("traverse[%d].max_depth", index),
			Message: "max_depth must be non-negative",
		}
	}

	return nil
}

// Validate checks if a derived collection is valid
func (d *Derived) Validate(viewName, derivedName string) error {
	if d.Source == "" {
		return &ValidationError{
			View:    viewName,
			Field:   fmt.Sprintf("derived.%s.source", derivedName),
			Message: "source is required",
		}
	}

	// Must have at least one operation: group_by, where, or embed
	if d.GroupBy == "" && d.Where == "" && len(d.Embed) == 0 {
		return &ValidationError{
			View:    viewName,
			Field:   fmt.Sprintf("derived.%s", derivedName),
			Message: "must specify at least one of: group_by, where, embed",
		}
	}

	return nil
}

// Validate checks if a relation export is valid
func (r *RelationExport) Validate(meta *metamodel.Metamodel, viewName string, index int) error {
	if len(r.Types) == 0 {
		return &ValidationError{
			View:    viewName,
			Field:   fmt.Sprintf("relation_exports[%d].types", index),
			Message: "types is required",
		}
	}

	// Validate relation types exist
	for _, relType := range r.Types {
		if _, ok := meta.GetRelationDef(relType); !ok {
			return &ValidationError{
				View:    viewName,
				Field:   fmt.Sprintf("relation_exports[%d].types", index),
				Message: fmt.Sprintf("unknown relation type: %s", relType),
			}
		}
	}

	if r.CollectAs == "" {
		return &ValidationError{
			View:    viewName,
			Field:   fmt.Sprintf("relation_exports[%d].collect_as", index),
			Message: "collect_as is required",
		}
	}

	return nil
}

// ValidationError represents a validation error in a view definition
type ValidationError struct {
	View    string
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("view %s: %s: %s", e.View, e.Field, e.Message)
}
