// Package pattern provides glob pattern parsing, validation, and regex conversion.
// This package has no dependencies on other internal packages to avoid circular imports.
package pattern

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// MaxWildcards is the maximum number of wildcards allowed in a pattern
// to prevent ReDoS attacks via excessive regex backtracking.
const MaxWildcards = 10

// ErrPatternTooComplex is returned when a pattern contains too many wildcards.
var ErrPatternTooComplex = errors.New("pattern too complex")

// ErrLeadingWildcard is returned when a pattern starts with an unescaped wildcard.
var ErrLeadingWildcard = errors.New("leading wildcards not supported")

// Error provides details about why a pattern was rejected.
type Error struct {
	Pattern string
	Reason  string
	Err     error
}

// Error returns the error message.
func (e *Error) Error() string {
	return fmt.Sprintf("pattern %q: %s", e.Pattern, e.Reason)
}

// Unwrap returns the underlying error type.
func (e *Error) Unwrap() error {
	return e.Err
}

// Validate checks if a glob pattern is valid and safe to use.
// It rejects patterns with more than MaxWildcards (10) unescaped wildcards
// to prevent ReDoS attacks.
func Validate(pattern string) error {
	wildcardCount := 0
	runes := []rune(pattern)

	for i := 0; i < len(runes); i++ {
		r := runes[i]

		// Handle escape sequences - skip the next character
		if r == '\\' && i+1 < len(runes) {
			next := runes[i+1]
			if next == '*' || next == '?' || next == '~' || next == '\\' {
				i++ // Skip the escaped character
				continue
			}
		}

		// Count unescaped wildcards
		if r == '*' || r == '?' {
			wildcardCount++
		}
	}

	if wildcardCount > MaxWildcards {
		return &Error{
			Pattern: pattern,
			Reason:  fmt.Sprintf("exceeds maximum of %d wildcards (found %d)", MaxWildcards, wildcardCount),
			Err:     ErrPatternTooComplex,
		}
	}

	return nil
}

// GlobToAnchoredRegex converts a glob pattern to an anchored regex pattern (^...$).
// Use this for full-string matching (e.g., filter expressions).
//
// It uses rune iteration to properly handle Unicode characters:
//   - * matches zero or more Unicode codepoints
//   - ? matches exactly one Unicode codepoint
//   - \* matches a literal asterisk
//   - \? matches a literal question mark
//   - \~ matches a literal tilde
//   - \\ matches a literal backslash
//
// All other characters are escaped for regex safety.
func GlobToAnchoredRegex(glob string) string {
	return "^" + globToRegexCore(glob) + "$"
}

// GlobToSubstringRegex converts a glob pattern to an unanchored regex pattern.
// Use this for substring matching (e.g., search queries).
func GlobToSubstringRegex(glob string) string {
	return globToRegexCore(glob)
}

// globToRegexCore is the shared implementation for glob-to-regex conversion.
func globToRegexCore(glob string) string {
	var result strings.Builder

	runes := []rune(glob)
	for i := 0; i < len(runes); i++ {
		r := runes[i]

		switch r {
		case '\\':
			// Handle escape sequences
			if i+1 < len(runes) {
				next := runes[i+1]
				if next == '*' || next == '?' || next == '~' || next == '\\' {
					// Escaped special char - write literal
					result.WriteString(regexp.QuoteMeta(string(next)))
					i++ // Skip next rune
					continue
				}
			}
			// Lone backslash or backslash before non-special char - escape it
			result.WriteString("\\\\")

		case '*':
			// Match zero or more Unicode codepoints
			result.WriteString(".*")

		case '?':
			// Match exactly one Unicode codepoint
			result.WriteString(".")

		default:
			// Escape regex special characters and write Unicode chars safely
			result.WriteString(regexp.QuoteMeta(string(r)))
		}
	}

	return result.String()
}

// ContainsWildcard returns true if the pattern contains unescaped * or ? characters.
func ContainsWildcard(s string) bool {
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		r := runes[i]

		// Handle escape sequences - skip the next character
		if r == '\\' && i+1 < len(runes) {
			next := runes[i+1]
			if next == '*' || next == '?' || next == '~' || next == '\\' {
				i++ // Skip escaped char
				continue
			}
		}

		if r == '*' || r == '?' {
			return true
		}
	}
	return false
}

// StartsWithWildcard checks if pattern begins with an unescaped * or ?.
func StartsWithWildcard(pattern string) bool {
	runes := []rune(pattern)
	if len(runes) == 0 {
		return false
	}
	// First character must be * or ? (not escaped - can't escape first char)
	r := runes[0]
	return r == '*' || r == '?'
}

// SplitFuzzyWildcard splits a fuzzy+wildcard pattern at the first unescaped wildcard.
// Returns (fuzzyPart, wildcardPart, hasWildcard).
// Example: "foo*" -> ("foo", "*", true)
// Example: "test\\*val*" -> ("test*val", "*", true) - escaped * preserved in fuzzy part
func SplitFuzzyWildcard(pattern string) (fuzzyPart, wildcardPart string, hasWildcard bool) {
	runes := []rune(pattern)
	fuzzyRunes := make([]rune, 0, len(runes))

	for i := 0; i < len(runes); i++ {
		r := runes[i]

		// Handle escape sequences
		if r == '\\' && i+1 < len(runes) {
			next := runes[i+1]
			if next == '*' || next == '?' || next == '~' || next == '\\' {
				// Keep the literal character in fuzzy part (without backslash)
				fuzzyRunes = append(fuzzyRunes, next)
				i++ // Skip next rune
				continue
			}
		}

		// Unescaped wildcard - this is the split point
		if r == '*' || r == '?' {
			return string(fuzzyRunes), string(runes[i:]), true
		}

		fuzzyRunes = append(fuzzyRunes, r)
	}

	// No wildcard found
	return string(fuzzyRunes), "", false
}

// Compile validates and compiles a glob pattern.
// It returns:
//   - A compiled regex if the pattern contains unescaped wildcards
//   - isGlob=true if wildcards were found
//   - An error if the pattern is invalid or too complex
func Compile(pat string) (*regexp.Regexp, bool, error) {
	// First validate the pattern for safety
	if err := Validate(pat); err != nil {
		return nil, ContainsWildcard(pat), err
	}

	// Check if this is actually a glob pattern
	isGlob := ContainsWildcard(pat)
	if !isGlob {
		return nil, false, nil
	}

	// Convert to regex and compile
	regexPattern := GlobToAnchoredRegex(pat)
	re, err := regexp.Compile(regexPattern)
	if err != nil {
		return nil, true, fmt.Errorf("failed to compile glob pattern %q: %w", pat, err)
	}

	return re, true, nil
}

// CompileSubstring validates and compiles a glob pattern for substring matching.
// Unlike Compile, this produces an unanchored regex.
func CompileSubstring(pat string) (*regexp.Regexp, error) {
	// Validate the pattern for safety
	if err := Validate(pat); err != nil {
		return nil, err
	}

	// Check for leading wildcards (performance concern)
	if StartsWithWildcard(pat) {
		return nil, &Error{
			Pattern: pat,
			Reason:  "leading wildcards cause performance issues",
			Err:     ErrLeadingWildcard,
		}
	}

	// Convert to unanchored regex and compile (case-insensitive)
	regexPattern := "(?i)" + GlobToSubstringRegex(pat)
	re, err := regexp.Compile(regexPattern)
	if err != nil {
		return nil, fmt.Errorf("failed to compile pattern %q: %w", pat, err)
	}

	return re, nil
}
