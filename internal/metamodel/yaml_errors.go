package metamodel

import (
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// yamlTagNames maps YAML !!tags to human-readable descriptions.
var yamlTagNames = map[string]string{
	"!!str":   "a string",
	"!!seq":   "a list",
	"!!map":   "a mapping",
	"!!int":   "a number",
	"!!float": "a number",
	"!!bool":  "a boolean",
	"!!null":  "null",
}

// goTypeDescriptions maps Go type strings to human-readable descriptions.
var goTypeDescriptions = map[string]string{
	"[]string": "a list",
	"bool":     "true/false",
	"int":      "a number",
	"*int":     "a number",
	"string":   "a string",
	"float64":  "a number",
}

// humanizeYAMLError rewrites yaml.TypeError messages to be more user-friendly.
// For non-yaml.TypeError errors, it returns the original error unchanged.
func humanizeYAMLError(err error) error {
	var typeErr *yaml.TypeError
	if !errors.As(err, &typeErr) {
		return err
	}

	humanized := make([]string, 0, len(typeErr.Errors))
	for _, msg := range typeErr.Errors {
		humanized = append(humanized, humanizeUnmarshalError(msg))
	}

	if len(humanized) == 1 {
		return fmt.Errorf("%s", humanized[0])
	}
	return fmt.Errorf("yaml: unmarshal errors:\n  %s", strings.Join(humanized, "\n  "))
}

// humanizeUnmarshalError converts a single YAML unmarshal error message
// into a more readable form. It parses the message structure rather than
// relying on regex to handle edge cases (backtick values containing newlines,
// custom YAML tags, etc.).
//
// Expected input format from yaml library:
//
//	"line <N>: cannot unmarshal <tag> `<value>` into <GoType>"
//	"line <N>: cannot unmarshal <tag> into <GoType>"
func humanizeUnmarshalError(msg string) string {
	parsed := parseUnmarshalMsg(msg)
	if parsed.goType == "" {
		// Could not extract the Go type — return a safe generic message
		return sanitizeErrorMessage(msg)
	}

	gotDesc := describeYAMLTag(parsed.yamlTag)
	expectedDesc := describeGoType(parsed.goType)

	if parsed.lineNum != "" {
		return fmt.Sprintf("line %s: expected %s, got %s", parsed.lineNum, expectedDesc, gotDesc)
	}
	return fmt.Sprintf("expected %s, got %s", expectedDesc, gotDesc)
}

// unmarshalParts holds the parsed components of a YAML unmarshal error message.
type unmarshalParts struct {
	lineNum string // e.g., "7", or "" if no line number
	yamlTag string // e.g., "!!str", "!!seq", "!custom"
	goType  string // e.g., "[]string", "metamodel.EntityDef"
}

// parseUnmarshalMsg extracts structured parts from a YAML unmarshal error message.
// It handles all variations: with/without line numbers, with/without backtick values,
// values containing newlines or special characters, custom YAML tags, etc.
func parseUnmarshalMsg(msg string) unmarshalParts {
	var parts unmarshalParts
	remaining := msg

	// Step 1: Extract optional "line <N>: " prefix
	if strings.HasPrefix(remaining, "line ") {
		remaining = remaining[5:]
		colonIdx := strings.Index(remaining, ": ")
		if colonIdx > 0 {
			parts.lineNum = remaining[:colonIdx]
			remaining = remaining[colonIdx+2:]
		}
	}

	// Step 2: Expect "cannot unmarshal "
	const unmarshalPrefix = "cannot unmarshal "
	if !strings.HasPrefix(remaining, unmarshalPrefix) {
		return parts
	}
	remaining = remaining[len(unmarshalPrefix):]

	// Step 3: Extract the YAML tag (starts with !, continues until whitespace)
	tagEnd := 0
	for tagEnd < len(remaining) && remaining[tagEnd] != ' ' && remaining[tagEnd] != '\t' && remaining[tagEnd] != '\n' {
		tagEnd++
	}
	if tagEnd == 0 {
		return parts
	}
	parts.yamlTag = remaining[:tagEnd]
	remaining = remaining[tagEnd:]

	// Step 4: Extract the Go type from after the last " into " in the remainder.
	// The remainder may contain: ` <whitespace> `<value>` into <GoType> `
	// The value in backticks can contain anything (newlines, "into", etc.),
	// so we find the LAST " into " to get the Go type reliably.
	const intoSep = " into "
	if idx := strings.LastIndex(remaining, intoSep); idx >= 0 {
		parts.goType = remaining[idx+len(intoSep):]
	} else if trimmed := strings.TrimSpace(remaining); strings.HasPrefix(trimmed, "into ") {
		parts.goType = trimmed[5:]
	}

	return parts
}

// sanitizeErrorMessage removes Go/YAML internals from an error message
// that couldn't be fully parsed. This is a safety net.
func sanitizeErrorMessage(msg string) string {
	// Try to at least extract line number and give a generic message
	if strings.HasPrefix(msg, "line ") {
		colonIdx := strings.Index(msg, ": ")
		if colonIdx > 0 {
			lineNum := msg[5:colonIdx]
			return fmt.Sprintf("line %s: invalid value for this field", lineNum)
		}
	}
	return "invalid YAML structure"
}

// describeYAMLTag returns a human-readable description for a YAML tag.
// Falls back to "an unexpected value" for unknown tags to avoid leaking YAML internals.
func describeYAMLTag(tag string) string {
	if desc, ok := yamlTagNames[tag]; ok {
		return desc
	}
	return "an unexpected value"
}

// describeGoType returns a human-readable description of a Go type string.
func describeGoType(goType string) string {
	// Exact matches first
	if desc, ok := goTypeDescriptions[goType]; ok {
		return desc
	}
	// Prefix/pattern matches
	if strings.HasPrefix(goType, "[]") {
		return "a list"
	}
	if strings.HasPrefix(goType, "map[") {
		return "a mapping"
	}
	// Struct types like "metamodel.Metamodel" or "metamodel.EntityDef"
	if strings.Contains(goType, ".") {
		return "a mapping"
	}
	// Unknown Go type — use generic description to avoid leaking internals
	return "a valid value"
}
