package conflict

import (
	"fmt"
	"os"

	"github.com/Sourcehaven-BV/rela/internal/markdown"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// Resolve applies a resolution to a conflicted file and returns the resolved entity or relation.
func Resolve(cf *ConflictedFile, resolution *Resolution) (*model.Entity, *model.Relation, error) {
	if cf.Ours == nil || cf.Theirs == nil {
		return nil, nil, fmt.Errorf("cannot resolve: both sides must be parsed")
	}

	// Handle entity files
	if cf.Ours.Entity != nil && cf.Theirs.Entity != nil {
		entity, err := resolveEntity(cf, resolution)
		if err != nil {
			return nil, nil, err
		}
		return entity, nil, nil
	}

	// Handle relation files
	if cf.Ours.Relation != nil && cf.Theirs.Relation != nil {
		relation, err := resolveRelation(cf, resolution)
		if err != nil {
			return nil, nil, err
		}
		return nil, relation, nil
	}

	return nil, nil, fmt.Errorf("cannot resolve: mixed or unparseable content")
}

// resolveEntity merges two entity versions based on the resolution.
func resolveEntity(cf *ConflictedFile, resolution *Resolution) (*model.Entity, error) {
	ours := cf.Ours.Entity
	theirs := cf.Theirs.Entity

	// Start with a copy of one side as the base
	resolved := &model.Entity{
		ID:         ours.ID,
		Type:       ours.Type,
		Properties: make(map[string]interface{}),
		FilePath:   cf.Path,
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
		case SideOurs:
			fallthrough
		default:
			// Default to ours
			if val, ok := ours.Properties[prop]; ok {
				resolved.Properties[prop] = val
			}
		}
	}

	// Resolve content
	if resolution.ManualContent != "" {
		resolved.Content = resolution.ManualContent
	} else if resolution.ContentChoice == SideTheirs {
		resolved.Content = theirs.Content
	} else {
		resolved.Content = ours.Content
	}

	return resolved, nil
}

// resolveRelation merges two relation versions based on the resolution.
func resolveRelation(cf *ConflictedFile, resolution *Resolution) (*model.Relation, error) {
	ours := cf.Ours.Relation
	theirs := cf.Theirs.Relation

	resolved := &model.Relation{
		From:       ours.From,
		Type:       ours.Type,
		To:         ours.To,
		Properties: make(map[string]interface{}),
		FilePath:   cf.Path,
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
		case SideOurs:
			fallthrough
		default:
			if val, ok := ours.Properties[prop]; ok {
				resolved.Properties[prop] = val
			}
		}
	}

	// Resolve content
	if resolution.ManualContent != "" {
		resolved.Content = resolution.ManualContent
	} else if resolution.ContentChoice == SideTheirs {
		resolved.Content = theirs.Content
	} else {
		resolved.Content = ours.Content
	}

	return resolved, nil
}

// ResolveAndWrite resolves a conflict and writes the result to disk.
func ResolveAndWrite(cf *ConflictedFile, resolution *Resolution, meta *metamodel.Metamodel) error {
	entity, relation, err := Resolve(cf, resolution)
	if err != nil {
		return err
	}

	fio := markdown.NewFileIO(storage.NewOsFS())

	if entity != nil {
		// Validate entity before writing
		if meta != nil {
			if errs := meta.ValidateEntity(entity); len(errs) > 0 {
				return fmt.Errorf("validation failed: %v", errs[0])
			}
		}
		return fio.WriteEntity(entity, cf.Path)
	}

	if relation != nil {
		return fio.WriteRelation(relation, cf.Path)
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
func WriteResolved(path string, entity *model.Entity, relation *model.Relation) error {
	fio := markdown.NewFileIO(storage.NewOsFS())

	if entity != nil {
		return fio.WriteEntity(entity, path)
	}
	if relation != nil {
		return fio.WriteRelation(relation, path)
	}
	return fmt.Errorf("nothing to write")
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
