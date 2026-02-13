package transclusion

import "strings"

// EntityNotFoundError is returned when a referenced entity doesn't exist.
type EntityNotFoundError struct {
	ID             string
	ReferencedFrom string
}

func (e *EntityNotFoundError) Error() string {
	if e.ReferencedFrom != "" {
		return "transclusion target not found: " + e.ID + " (referenced from " + e.ReferencedFrom + ")"
	}
	return "transclusion target not found: " + e.ID
}

// SectionNotFoundError is returned when a referenced section doesn't exist.
type SectionNotFoundError struct {
	EntityID string
	Section  string
}

func (e *SectionNotFoundError) Error() string {
	return "section not found: \"" + e.Section + "\" in entity " + e.EntityID
}

// CircularTransclusionError is returned when a circular transclusion is detected.
type CircularTransclusionError struct {
	Chain []string // e.g., ["REQ-001", "REQ-002", "REQ-001"]
}

func (e *CircularTransclusionError) Error() string {
	return "circular transclusion detected: " + strings.Join(e.Chain, " → ")
}

// MaxDepthExceededError is returned when transclusion depth exceeds the limit.
type MaxDepthExceededError struct {
	MaxDepth int
	EntityID string
}

func (e *MaxDepthExceededError) Error() string {
	return "maximum transclusion depth exceeded at entity " + e.EntityID
}
