// Package conflict provides detection and resolution of git merge conflicts
// in entity and relation markdown files.
package conflict

import (
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// Side represents which side of a conflict to use.
type Side string

const (
	SideOurs   Side = "ours"   // Current branch (HEAD)
	SideTheirs Side = "theirs" // Incoming branch
)

// Marker represents a single conflict region in a file.
type Marker struct {
	StartLine int    // Line number of <<<<<<<
	MidLine   int    // Line number of =======
	EndLine   int    // Line number of >>>>>>>
	OursRef   string // Reference after <<<<<<< (e.g., "HEAD")
	TheirsRef string // Reference after >>>>>>> (e.g., "feature-branch")
}

// ConflictedFile represents a file with one or more git conflicts.
type ConflictedFile struct {
	Path       string      // Absolute path to the file
	EntityType string      // Entity type (if known)
	EntityID   string      // Entity ID (if parseable from either side)
	Markers    []Marker    // All conflict regions in the file
	Ours       *ParsedSide // Parsed content from "ours" side
	Theirs     *ParsedSide // Parsed content from "theirs" side
}

// ParsedSide represents one side of a conflict, parsed into structured data.
type ParsedSide struct {
	Entity   *entity.Entity   // Parsed entity (for entity files)
	Relation *entity.Relation // Parsed relation (for relation files)
	Raw      string          // Raw content of this side
}

// PropertyDiff represents the difference in a single property between sides.
type PropertyDiff struct {
	Property    string      // Property name
	OursValue   interface{} // Value on "ours" side (nil if not present)
	TheirsValue interface{} // Value on "theirs" side (nil if not present)
	IsSame      bool        // True if both sides have the same value
}

// Info provides a structured view of differences between sides.
type Info struct {
	File              *ConflictedFile
	PropertyDiffs     []PropertyDiff // Differences in frontmatter properties
	ContentDiffOurs   string         // Markdown content from ours
	ContentDiffTheirs string         // Markdown content from theirs
	ContentSame       bool           // True if content is identical
}

// Resolution represents how to resolve a conflict.
type Resolution struct {
	PropertyChoices map[string]Side // Which side to use for each property
	ContentChoice   Side            // Which side to use for content
	ManualContent   string          // If set, use this instead of either side
}

// DetectResult contains the results of scanning for conflicts.
type DetectResult struct {
	Files []ConflictedFile // All files with conflicts
}
