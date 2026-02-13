package transclusion

import (
	"bufio"
	"regexp"
	"strings"
)

// headerPattern matches markdown headers (# through ######)
var headerPattern = regexp.MustCompile(`^(#{1,6})\s+(.+)$`)

// ExtractSection extracts the content under a specific heading from markdown content.
// The heading match is case-insensitive.
// Returns the section content including the heading itself.
// If the heading is not found, returns an empty string and false.
func ExtractSection(content, heading string) (string, bool) {
	if heading == "" {
		return content, true
	}

	scanner := bufio.NewScanner(strings.NewReader(content))
	var result strings.Builder
	var inSection bool
	var sectionLevel int
	headingLower := strings.ToLower(strings.TrimSpace(heading))

	for scanner.Scan() {
		line := scanner.Text()

		//nolint:nestif // Nested ifs needed for section start/end detection with header level tracking
		if match := headerPattern.FindStringSubmatch(line); match != nil {
			level := len(match[1])
			title := strings.TrimSpace(match[2])
			titleLower := strings.ToLower(title)

			if inSection {
				// Check if this header ends our section
				// (same or higher level = end of section)
				if level <= sectionLevel {
					break
				}
			} else {
				// Check if this is the header we're looking for
				if titleLower == headingLower {
					inSection = true
					sectionLevel = level
				}
			}
		}

		if inSection {
			result.WriteString(line)
			result.WriteString("\n")
		}
	}

	if !inSection {
		return "", false
	}

	return strings.TrimSuffix(result.String(), "\n"), true
}

// ExtractSectionContent extracts only the body content under a heading,
// without the heading itself.
func ExtractSectionContent(content, heading string) (string, bool) {
	section, found := ExtractSection(content, heading)
	if !found {
		return "", false
	}

	// Remove the first line (the heading)
	lines := strings.SplitN(section, "\n", 2)
	if len(lines) < 2 {
		return "", true // Section exists but has no content
	}

	return strings.TrimPrefix(lines[1], "\n"), true
}

// Header level constants.
const (
	minHeaderLevel = 1
	maxHeaderLevel = 6
)

// AdjustHeaderLevels increases or decreases all header levels in content by delta.
// Positive delta increases levels (e.g., # becomes ##), negative decreases.
// Headers are capped at level 6 (max) and 1 (min).
func AdjustHeaderLevels(content string, delta int) string {
	if delta == 0 {
		return content
	}

	scanner := bufio.NewScanner(strings.NewReader(content))
	var result strings.Builder
	first := true

	for scanner.Scan() {
		line := scanner.Text()

		if !first {
			result.WriteString("\n")
		}
		first = false

		if match := headerPattern.FindStringSubmatch(line); match != nil {
			level := len(match[1])
			newLevel := level + delta

			// Clamp to valid range
			if newLevel < minHeaderLevel {
				newLevel = minHeaderLevel
			}
			if newLevel > maxHeaderLevel {
				newLevel = maxHeaderLevel
			}

			result.WriteString(strings.Repeat("#", newLevel))
			result.WriteString(" ")
			result.WriteString(match[2])
		} else {
			result.WriteString(line)
		}
	}

	return result.String()
}

// GetHeaderLevel returns the header level of the first line if it's a header,
// or 0 if the content doesn't start with a header.
func GetHeaderLevel(content string) int {
	firstLine := strings.SplitN(content, "\n", 2)[0]
	if match := headerPattern.FindStringSubmatch(firstLine); match != nil {
		return len(match[1])
	}
	return 0
}

// StripFirstHeader removes the first header line from content if present.
func StripFirstHeader(content string) string {
	firstLine := strings.SplitN(content, "\n", 2)[0]
	if headerPattern.MatchString(firstLine) {
		parts := strings.SplitN(content, "\n", 2)
		if len(parts) < 2 {
			return ""
		}
		return strings.TrimPrefix(parts[1], "\n")
	}
	return content
}
