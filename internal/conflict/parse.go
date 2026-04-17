package conflict

import (
	"fmt"
	"os"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/markdown"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/natsort"
)

// ParseConflictedFile reads and parses both sides of a conflicted file.
func ParseConflictedFile(path string, meta *metamodel.Metamodel) (*ConflictedFile, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	return ParseConflictedContent(path, string(content), meta)
}

// ParseConflictedContent parses conflicted content into structured data.
func ParseConflictedContent(path, content string, meta *metamodel.Metamodel) (*ConflictedFile, error) {
	markers := FindMarkers(content)
	if len(markers) == 0 {
		return nil, fmt.Errorf("no conflict markers found in %s", path)
	}

	// Extract ours and theirs content
	oursContent, theirsContent := ExtractSides(content, markers)

	cf := &ConflictedFile{
		Path:    path,
		Markers: markers,
		Ours:    &ParsedSide{Raw: oursContent},
		Theirs:  &ParsedSide{Raw: theirsContent},
	}

	// Parse ours side
	if doc, err := markdown.ParseDocument(oursContent); err == nil {
		if isRelationFile(path) {
			cf.Ours.Relation = docToRelation(doc, path)
		} else {
			cf.Ours.Entity = docToEntity(doc, path, meta)
			if cf.Ours.Entity != nil {
				cf.EntityID = cf.Ours.Entity.ID
				cf.EntityType = cf.Ours.Entity.Type
			}
		}
	}

	// Parse theirs side
	if doc, err := markdown.ParseDocument(theirsContent); err == nil {
		if isRelationFile(path) {
			cf.Theirs.Relation = docToRelation(doc, path)
		} else {
			cf.Theirs.Entity = docToEntity(doc, path, meta)
			if cf.Theirs.Entity != nil && cf.EntityID == "" {
				cf.EntityID = cf.Theirs.Entity.ID
				cf.EntityType = cf.Theirs.Entity.Type
			}
		}
	}

	return cf, nil
}

// ExtractSides extracts the "ours" and "theirs" versions from conflicted content.
// It handles multiple conflict regions by merging them appropriately.
func ExtractSides(content string, markers []Marker) (ours, theirs string) {
	lines := strings.Split(content, "\n")

	var oursLines, theirsLines []string
	inConflict := false
	inOurs := false
	conflictIdx := 0

	for i, line := range lines {
		lineNum := i + 1

		if conflictIdx < len(markers) && lineNum == markers[conflictIdx].StartLine {
			inConflict = true
			inOurs = true
			continue
		}

		if inConflict && conflictIdx < len(markers) && lineNum == markers[conflictIdx].MidLine {
			inOurs = false
			continue
		}

		if inConflict && conflictIdx < len(markers) && lineNum == markers[conflictIdx].EndLine {
			inConflict = false
			inOurs = false
			conflictIdx++
			continue
		}

		if inConflict {
			if inOurs {
				oursLines = append(oursLines, line)
			} else {
				theirsLines = append(theirsLines, line)
			}
		} else {
			// Outside conflict - add to both sides
			oursLines = append(oursLines, line)
			theirsLines = append(theirsLines, line)
		}
	}

	return strings.Join(oursLines, "\n"), strings.Join(theirsLines, "\n")
}

// AnalyzeConflict creates a detailed diff between the two sides.
func AnalyzeConflict(cf *ConflictedFile) *Info {
	info := &Info{
		File:              cf,
		PropertyDiffs:     make([]PropertyDiff, 0),
		ContentDiffOurs:   "",
		ContentDiffTheirs: "",
		ContentSame:       true,
	}

	// Get properties and content from each side
	var oursProps, theirsProps map[string]interface{}
	var oursContent, theirsContent string

	if cf.Ours != nil {
		if cf.Ours.Entity != nil {
			oursProps = cf.Ours.Entity.Properties
			oursContent = cf.Ours.Entity.Content
		} else if cf.Ours.Relation != nil {
			oursProps = cf.Ours.Relation.Properties
			oursContent = cf.Ours.Relation.Content
		}
	}

	if cf.Theirs != nil {
		if cf.Theirs.Entity != nil {
			theirsProps = cf.Theirs.Entity.Properties
			theirsContent = cf.Theirs.Entity.Content
		} else if cf.Theirs.Relation != nil {
			theirsProps = cf.Theirs.Relation.Properties
			theirsContent = cf.Theirs.Relation.Content
		}
	}

	// Collect all property keys
	allKeys := make(map[string]bool)
	for k := range oursProps {
		allKeys[k] = true
	}
	for k := range theirsProps {
		allKeys[k] = true
	}

	// Sort property keys for stable ordering
	sortedKeys := make([]string, 0, len(allKeys))
	for k := range allKeys {
		sortedKeys = append(sortedKeys, k)
	}
	natsort.Strings(sortedKeys)

	// Compare properties (in sorted order)
	for _, prop := range sortedKeys {
		oursVal := oursProps[prop]
		theirsVal := theirsProps[prop]
		isSame := fmt.Sprintf("%v", oursVal) == fmt.Sprintf("%v", theirsVal)

		info.PropertyDiffs = append(info.PropertyDiffs, PropertyDiff{
			Property:    prop,
			OursValue:   oursVal,
			TheirsValue: theirsVal,
			IsSame:      isSame,
		})
	}

	// Compare content
	info.ContentDiffOurs = oursContent
	info.ContentDiffTheirs = theirsContent
	info.ContentSame = oursContent == theirsContent

	return info
}

// isRelationFile checks if a path is a relation file based on naming convention.
func isRelationFile(path string) bool {
	// Relations are stored as FROM--type--TO.md
	base := strings.TrimSuffix(path, ".md")
	parts := strings.Split(base, "--")
	return len(parts) == 3
}

// docToEntity converts a parsed document to an entity.
func docToEntity(doc *markdown.Document, _ string, meta *metamodel.Metamodel) *entity.Entity {
	id := doc.GetString("id")
	entityType := doc.GetString("type")

	if entityType == "" && meta != nil && id != "" {
		entityType = meta.InferEntityType(id)
	}

	if meta != nil && entityType != "" {
		entityType = meta.ResolveAlias(entityType)
	}

	e := &entity.Entity{
		ID:         id,
		Type:       entityType,
		Properties: make(map[string]interface{}),
		Content:    doc.Content,
	}

	for key, value := range doc.Frontmatter {
		if key != "id" && key != "type" {
			e.Properties[key] = value
		}
	}

	return e
}

// docToRelation converts a parsed document to a relation.
func docToRelation(doc *markdown.Document, _ string) *entity.Relation {
	relation := &entity.Relation{
		From:       doc.GetString("from"),
		Type:       doc.GetString("relation"),
		To:         doc.GetString("to"),
		Content:    doc.Content,
		Properties: make(map[string]interface{}),
	}

	for key, value := range doc.Frontmatter {
		if key != "from" && key != "relation" && key != "to" {
			relation.Properties[key] = value
		}
	}

	return relation
}
