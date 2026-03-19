package filter

import (
	"regexp"

	"github.com/Sourcehaven-BV/rela/internal/pattern"
)

// Re-export pattern package constants for backward compatibility
const MaxWildcards = pattern.MaxWildcards

// Re-export pattern package errors for backward compatibility
var (
	ErrPatternTooComplex = pattern.ErrPatternTooComplex
	ErrLeadingWildcard   = pattern.ErrLeadingWildcard
)

// PatternError is re-exported for backward compatibility
type PatternError = pattern.Error

// ValidatePattern checks if a glob pattern is valid and safe to use.
// It rejects patterns with more than MaxWildcards (10) unescaped wildcards.
func ValidatePattern(pat string) error {
	return pattern.Validate(pat)
}

// GlobToRegex converts a glob pattern to an anchored regex pattern (^...$).
// Use this for full-string matching in filter expressions.
func GlobToRegex(glob string) string {
	return pattern.GlobToAnchoredRegex(glob)
}

// ContainsUnescapedGlob returns true if the pattern contains unescaped * or ? characters.
func ContainsUnescapedGlob(s string) bool {
	return pattern.ContainsWildcard(s)
}

// SplitFuzzyWildcard splits a fuzzy+wildcard pattern at the first unescaped wildcard.
func SplitFuzzyWildcard(pat string) (fuzzyPart, wildcardPart string, hasWildcard bool) {
	return pattern.SplitFuzzyWildcard(pat)
}

// ParsePattern validates and compiles a glob pattern.
func ParsePattern(pat string) (*regexp.Regexp, bool, error) {
	return pattern.Compile(pat)
}
