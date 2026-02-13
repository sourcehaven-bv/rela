package transclusion

import (
	"regexp"
	"strings"
)

// transclusionPattern matches ![[EntityID]] or ![[EntityID#Section]]
// It captures the entity ID and optional section.
var transclusionPattern = regexp.MustCompile(`!\[\[([^\]#]+)(?:#([^\]]+))?\]\]`)

// Transclusion represents a parsed transclusion reference.
type Transclusion struct {
	EntityID string // Entity ID (e.g., "REQ-001")
	Section  string // Optional section heading (e.g., "Rationale")
	Raw      string // Original match text for replacement
	Start    int    // Start position in source text
	End      int    // End position in source text
}

// Parse finds all transclusion references in content.
// Returns transclusions in order of appearance.
func Parse(content string) []Transclusion {
	matches := transclusionPattern.FindAllStringSubmatchIndex(content, -1)
	if len(matches) == 0 {
		return nil
	}

	result := make([]Transclusion, 0, len(matches))
	for _, match := range matches {
		// Skip transclusions inside code blocks
		if isInsideCodeBlock(content, match[0]) {
			continue
		}

		t := Transclusion{
			Raw:   content[match[0]:match[1]],
			Start: match[0],
			End:   match[1],
		}

		// Extract entity ID (group 1)
		if match[2] >= 0 && match[3] >= 0 {
			t.EntityID = strings.TrimSpace(content[match[2]:match[3]])
		}

		// Extract section (group 2, optional)
		if match[4] >= 0 && match[5] >= 0 {
			t.Section = strings.TrimSpace(content[match[4]:match[5]])
		}

		if t.EntityID != "" {
			result = append(result, t)
		}
	}

	return result
}

// HasTransclusions returns true if content contains any transclusion references.
func HasTransclusions(content string) bool {
	return transclusionPattern.MatchString(content)
}

// isInsideCodeBlock checks if a position is inside a fenced code block.
func isInsideCodeBlock(content string, pos int) bool {
	// Count the number of ``` before this position
	// If odd, we're inside a code block
	prefix := content[:pos]
	count := 0
	i := 0
	for i < len(prefix) {
		if strings.HasPrefix(prefix[i:], "```") {
			count++
			i += 3
		} else {
			i++
		}
	}
	return count%2 == 1
}

// Replace replaces all transclusions in content using the provided replacer function.
// The replacer receives each Transclusion and returns the replacement text.
// Replacements are done from end to start to preserve positions.
func Replace(content string, replacer func(Transclusion) string) string {
	transclusions := Parse(content)
	if len(transclusions) == 0 {
		return content
	}

	// Replace from end to start to preserve positions
	result := content
	for i := len(transclusions) - 1; i >= 0; i-- {
		t := transclusions[i]
		replacement := replacer(t)
		result = result[:t.Start] + replacement + result[t.End:]
	}

	return result
}
