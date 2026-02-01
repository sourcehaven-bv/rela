package model

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// EntityID represents a parsed entity identifier
type EntityID struct {
	Prefix string // e.g., "REQ-", "DEC-"
	Number int    // e.g., 1, 42
	Raw    string // Original string
}

// Common ID pattern: PREFIX-NUMBER (e.g., REQ-001, DEC-42, ISO-CA-001)
var idPattern = regexp.MustCompile(`^([A-Za-z]+(?:-[A-Za-z]+)*-?)(\d+)$`)

// ParseEntityID parses an entity ID string into its components
func ParseEntityID(s string) (EntityID, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return EntityID{}, fmt.Errorf("empty entity ID")
	}

	matches := idPattern.FindStringSubmatch(s)
	if matches == nil {
		// Not a standard PREFIX-NUMBER format, treat as opaque ID
		return EntityID{Raw: s}, nil
	}

	num, err := strconv.Atoi(matches[2])
	if err != nil {
		return EntityID{}, fmt.Errorf("invalid number in ID: %s", s)
	}

	return EntityID{
		Prefix: matches[1],
		Number: num,
		Raw:    s,
	}, nil
}

// String returns the canonical string representation
func (id EntityID) String() string {
	if id.Raw != "" {
		return id.Raw
	}
	if id.Prefix != "" {
		return fmt.Sprintf("%s%d", id.Prefix, id.Number)
	}
	return fmt.Sprintf("%d", id.Number)
}

// FormatWithPadding returns the ID with zero-padded number
func (id EntityID) FormatWithPadding(width int) string {
	if id.Raw != "" && id.Prefix == "" {
		return id.Raw
	}
	return fmt.Sprintf("%s%0*d", id.Prefix, width, id.Number)
}

// NextID returns the next ID in sequence
func (id EntityID) NextID() EntityID {
	return EntityID{
		Prefix: id.Prefix,
		Number: id.Number + 1,
	}
}

// MatchesPattern checks if this ID matches a given prefix pattern
func (id EntityID) MatchesPattern(pattern string) bool {
	pattern = strings.TrimSuffix(pattern, "-")
	prefix := strings.TrimSuffix(id.Prefix, "-")
	return strings.EqualFold(prefix, pattern)
}

// ValidateID checks if a string is a valid entity ID
// It rejects IDs with path traversal attempts or invalid characters
func ValidateID(s string) error {
	if s == "" {
		return fmt.Errorf("empty entity ID")
	}

	// Reject path traversal
	if strings.Contains(s, "..") || strings.Contains(s, "/") || strings.Contains(s, "\\") {
		return fmt.Errorf("invalid characters in entity ID: %s", s)
	}

	// Allow alphanumeric, dash, underscore
	valid := regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	if !valid.MatchString(s) {
		return fmt.Errorf("invalid characters in entity ID: %s", s)
	}

	return nil
}

// ExtractHighestNumber finds the highest number for a given prefix from a list of IDs
func ExtractHighestNumber(ids []string, prefix string) int {
	highest := 0
	prefix = strings.ToUpper(strings.TrimSuffix(prefix, "-"))

	for _, idStr := range ids {
		id, err := ParseEntityID(idStr)
		if err != nil {
			continue
		}
		idPrefix := strings.ToUpper(strings.TrimSuffix(id.Prefix, "-"))
		if idPrefix == prefix && id.Number > highest {
			highest = id.Number
		}
	}

	return highest
}

// GenerateNextID generates the next available ID for a given prefix
func GenerateNextID(existingIDs []string, prefix string) string {
	highest := ExtractHighestNumber(existingIDs, prefix)
	// Ensure prefix ends with dash
	if !strings.HasSuffix(prefix, "-") {
		prefix += "-"
	}
	return fmt.Sprintf("%s%03d", strings.ToUpper(prefix), highest+1)
}
