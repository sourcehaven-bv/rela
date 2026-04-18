package conflict

import (
	"fmt"
	"os"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/markdown"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// Resolve applies a resolution to a conflicted file and returns the resolved entity or relation.
func Resolve(cf *ConflictedFile, resolution *Resolution) (*entity.Entity, *entity.Relation, error) {
	if cf.Ours == nil || cf.Theirs == nil {
		return nil, nil, fmt.Errorf("cannot resolve: both sides must be parsed")
	}

	// Handle entity files
	if cf.Ours.Entity != nil && cf.Theirs.Entity != nil {
		e := resolveEntity(cf, resolution)
		return e, nil, nil
	}

	// Handle relation files
	if cf.Ours.Relation != nil && cf.Theirs.Relation != nil {
		relation := resolveRelation(cf, resolution)
		return nil, relation, nil
	}

	return nil, nil, fmt.Errorf("cannot resolve: mixed or unparseable content")
}

// resolveEntity merges two entity versions based on the resolution.
func resolveEntity(cf *ConflictedFile, resolution *Resolution) *entity.Entity {
	ours := cf.Ours.Entity
	theirs := cf.Theirs.Entity

	// Start with a copy of one side as the base
	resolved := &entity.Entity{
		ID:         ours.ID,
		Type:       ours.Type,
		Properties: make(map[string]interface{}),
	}

	// If IDs differ, prefer theirs if explicitly chosen
	if ours.ID != theirs.ID {
		if resolution.PropertyChoices["id"] == SideTheirs {
			resolved.ID = theirs.ID
		}
	}

	// Merge properties based on resolution choices
	allProps := collectPropertyKeys(ours.Properties, theirs.Properties)
	for _, prop := range allProps {
		side := resolution.PropertyChoices[prop]
		switch side {
		case SideTheirs:
			if val, ok := theirs.Properties[prop]; ok {
				resolved.Properties[prop] = val
			}
		default:
			// Default to ours
			if val, ok := ours.Properties[prop]; ok {
				resolved.Properties[prop] = val
			}
		}
	}

	// Resolve content
	switch {
	case resolution.ManualContent != "":
		resolved.Content = resolution.ManualContent
	case resolution.ContentChoice == SideTheirs:
		resolved.Content = theirs.Content
	default:
		resolved.Content = ours.Content
	}

	return resolved
}

// resolveRelation merges two relation versions based on the resolution.
func resolveRelation(cf *ConflictedFile, resolution *Resolution) *entity.Relation {
	ours := cf.Ours.Relation
	theirs := cf.Theirs.Relation

	resolved := &entity.Relation{
		From:       ours.From,
		Type:       ours.Type,
		To:         ours.To,
		Properties: make(map[string]interface{}),
	}

	// Handle from/to/type differences
	if resolution.PropertyChoices["from"] == SideTheirs {
		resolved.From = theirs.From
	}
	if resolution.PropertyChoices["to"] == SideTheirs {
		resolved.To = theirs.To
	}
	if resolution.PropertyChoices["relation"] == SideTheirs {
		resolved.Type = theirs.Type
	}

	// Merge other properties
	allProps := collectPropertyKeys(ours.Properties, theirs.Properties)
	for _, prop := range allProps {
		side := resolution.PropertyChoices[prop]
		switch side {
		case SideTheirs:
			if val, ok := theirs.Properties[prop]; ok {
				resolved.Properties[prop] = val
			}
		default:
			if val, ok := ours.Properties[prop]; ok {
				resolved.Properties[prop] = val
			}
		}
	}

	// Resolve content
	switch {
	case resolution.ManualContent != "":
		resolved.Content = resolution.ManualContent
	case resolution.ContentChoice == SideTheirs:
		resolved.Content = theirs.Content
	default:
		resolved.Content = ours.Content
	}

	return resolved
}

// ResolveAndWrite resolves a conflict and writes the result to disk.
func ResolveAndWrite(cf *ConflictedFile, resolution *Resolution, meta *metamodel.Metamodel) error {
	e, relation, err := Resolve(cf, resolution)
	if err != nil {
		return err
	}

	if e != nil {
		// Validate entity before writing
		if meta != nil {
			if errs := meta.ValidateEntity(e.ID, e.Type, e.Properties); len(errs) > 0 {
				return fmt.Errorf("validation failed: %w", errs[0])
			}
		}
		return writeEntityFile(cf.Path, e)
	}

	if relation != nil {
		return writeRelationFile(cf.Path, relation)
	}

	return fmt.Errorf("nothing to write")
}

// AcceptOurs resolves a conflict by accepting all values from "ours" (HEAD).
func AcceptOurs(cf *ConflictedFile) *Resolution {
	resolution := &Resolution{
		PropertyChoices: make(map[string]Side),
		ContentChoice:   SideOurs,
	}

	// Set all properties to ours
	if cf.Ours != nil && cf.Ours.Entity != nil {
		for prop := range cf.Ours.Entity.Properties {
			resolution.PropertyChoices[prop] = SideOurs
		}
	}
	if cf.Theirs != nil && cf.Theirs.Entity != nil {
		for prop := range cf.Theirs.Entity.Properties {
			if _, ok := resolution.PropertyChoices[prop]; !ok {
				resolution.PropertyChoices[prop] = SideOurs
			}
		}
	}

	return resolution
}

// AcceptTheirs resolves a conflict by accepting all values from "theirs" (incoming).
func AcceptTheirs(cf *ConflictedFile) *Resolution {
	resolution := &Resolution{
		PropertyChoices: make(map[string]Side),
		ContentChoice:   SideTheirs,
	}

	// Set all properties to theirs
	if cf.Ours != nil && cf.Ours.Entity != nil {
		for prop := range cf.Ours.Entity.Properties {
			resolution.PropertyChoices[prop] = SideTheirs
		}
	}
	if cf.Theirs != nil && cf.Theirs.Entity != nil {
		for prop := range cf.Theirs.Entity.Properties {
			resolution.PropertyChoices[prop] = SideTheirs
		}
	}

	return resolution
}

// WriteResolved writes a resolved entity or relation to disk.
func WriteResolved(path string, e *entity.Entity, relation *entity.Relation) error {
	if e != nil {
		return writeEntityFile(path, e)
	}
	if relation != nil {
		return writeRelationFile(path, relation)
	}
	return fmt.Errorf("nothing to write")
}

// writeEntityFile formats and writes an entity to disk.
func writeEntityFile(path string, e *entity.Entity) error {
	fm := map[string]interface{}{"id": e.ID, "type": e.Type}
	for k, v := range e.Properties {
		fm[k] = v
	}
	content := e.Content
	if content != "" {
		content = markdown.FormatMarkdown(content)
	}
	formatted, err := markdown.FormatDocumentOrdered(fm, content, []string{"id", "type"})
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(formatted), 0644)
}

// writeRelationFile formats and writes a relation to disk.
func writeRelationFile(path string, r *entity.Relation) error {
	fm := map[string]interface{}{"from": r.From, "relation": r.Type, "to": r.To}
	for k, v := range r.Properties {
		fm[k] = v
	}
	content := r.Content
	if content != "" {
		content = markdown.FormatMarkdown(content)
	}
	formatted, err := markdown.FormatDocumentOrdered(fm, content, []string{"from", "relation", "to"})
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(formatted), 0644)
}

// RemoveConflictMarkers removes all conflict markers from a file, keeping the "ours" content.
// This is a simple text-based resolution that doesn't parse the YAML.
func RemoveConflictMarkers(path string, keepSide Side) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	markers := FindMarkers(string(content))
	if len(markers) == 0 {
		return nil // No conflicts to resolve
	}

	var resolved string
	if keepSide == SideTheirs {
		_, resolved = ExtractSides(string(content), markers)
	} else {
		resolved, _ = ExtractSides(string(content), markers)
	}

	return os.WriteFile(path, []byte(resolved), 0644)
}

// collectPropertyKeys returns all unique property keys from both maps.
func collectPropertyKeys(a, b map[string]interface{}) []string {
	keys := make(map[string]bool)
	for k := range a {
		keys[k] = true
	}
	for k := range b {
		keys[k] = true
	}

	result := make([]string, 0, len(keys))
	for k := range keys {
		result = append(result, k)
	}
	return result
}
