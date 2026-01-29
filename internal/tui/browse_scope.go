package tui

import "fmt"

// BrowseScope defines a subset of entities for sequential navigation.
// When a scope is present, the detail screen enables prev/next navigation
// within the scope and shows progress indicators.
type BrowseScope struct {
	IDs    []string // Ordered list of entity IDs to browse
	Index  int      // Current position in IDs (0-based)
	Label  string   // Human-readable description, e.g., "12 search results"
	Origin Screen   // Screen to return to when exiting browse mode
}

// NewBrowseScope creates a new browse scope from a list of entity IDs.
func NewBrowseScope(ids []string, label string, origin Screen) *BrowseScope {
	if len(ids) == 0 {
		return nil
	}
	return &BrowseScope{
		IDs:    ids,
		Index:  0,
		Label:  label,
		Origin: origin,
	}
}

// Current returns the entity ID at the current position.
func (s *BrowseScope) Current() string {
	if s == nil || len(s.IDs) == 0 || s.Index < 0 || s.Index >= len(s.IDs) {
		return ""
	}
	return s.IDs[s.Index]
}

// HasNext returns true if there is a next entity in the scope.
func (s *BrowseScope) HasNext() bool {
	if s == nil {
		return false
	}
	return s.Index < len(s.IDs)-1
}

// HasPrev returns true if there is a previous entity in the scope.
func (s *BrowseScope) HasPrev() bool {
	if s == nil {
		return false
	}
	return s.Index > 0
}

// Next advances to the next entity. Returns true if the move was successful.
func (s *BrowseScope) Next() bool {
	if s == nil || !s.HasNext() {
		return false
	}
	s.Index++
	return true
}

// Prev moves to the previous entity. Returns true if the move was successful.
func (s *BrowseScope) Prev() bool {
	if s == nil || !s.HasPrev() {
		return false
	}
	s.Index--
	return true
}

// Progress returns a progress string like "[3/12]".
func (s *BrowseScope) Progress() string {
	if s == nil || len(s.IDs) == 0 {
		return ""
	}
	return fmt.Sprintf("[%d/%d]", s.Index+1, len(s.IDs))
}

// Count returns the total number of entities in the scope.
func (s *BrowseScope) Count() int {
	if s == nil {
		return 0
	}
	return len(s.IDs)
}

// SetIndex sets the current position to a specific index.
// Returns false if the index is out of bounds.
func (s *BrowseScope) SetIndex(idx int) bool {
	if s == nil || idx < 0 || idx >= len(s.IDs) {
		return false
	}
	s.Index = idx
	return true
}

// IndexOf returns the index of the given entity ID, or -1 if not found.
func (s *BrowseScope) IndexOf(id string) int {
	if s == nil {
		return -1
	}
	for i, scopeID := range s.IDs {
		if scopeID == id {
			return i
		}
	}
	return -1
}
