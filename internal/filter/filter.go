package filter

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Operator represents a comparison operator
type Operator int

const (
	OpEqual        Operator = iota // =
	OpNotEqual                     // !=
	OpLess                         // <
	OpLessEqual                    // <=
	OpGreater                      // >
	OpGreaterEqual                 // >=
	OpRegex                        // =~
	OpFuzzy                        // ~
)

// String returns the string representation of an operator
func (o Operator) String() string {
	switch o {
	case OpEqual:
		return "="
	case OpNotEqual:
		return "!="
	case OpLess:
		return "<"
	case OpLessEqual:
		return "<="
	case OpGreater:
		return ">"
	case OpGreaterEqual:
		return ">="
	case OpRegex:
		return "=~"
	case OpFuzzy:
		return "~"
	default:
		return "?"
	}
}

// Filter represents a parsed filter expression
type Filter struct {
	Property    string
	Operator    Operator
	Value       string
	Regex       *regexp.Regexp // Compiled regex if Operator is OpRegex or IsGlob
	IsGlob      bool           // True if Value contains glob patterns (for string equality)
	FuzzyTarget string         // For OpFuzzy: fuzzy portion (before first wildcard)
	WildcardRe  *regexp.Regexp // For OpFuzzy: compiled wildcard suffix pattern (if any)
}

// Parse parses a filter string like "status=draft" or "valid_until<2025-02-01"
// Supported formats:
//   - property=value (equality, supports glob patterns with *)
//   - property!=value (not equal)
//   - property<value (less than)
//   - property<=value (less than or equal)
//   - property>value (greater than)
//   - property>=value (greater than or equal)
//   - property=~pattern (regex match)
func Parse(s string) (*Filter, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, errors.New("empty filter expression")
	}

	// Try operators in order of specificity (longest first)
	operators := []struct {
		str string
		op  Operator
	}{
		{"<=", OpLessEqual},
		{">=", OpGreaterEqual},
		{"!=", OpNotEqual},
		{"=~", OpRegex},
		{"~", OpFuzzy},
		{"<", OpLess},
		{">", OpGreater},
		{"=", OpEqual},
	}

	for _, op := range operators {
		idx := strings.Index(s, op.str)
		if idx <= 0 {
			continue
		}

		property := strings.TrimSpace(s[:idx])
		value := strings.TrimSpace(s[idx+len(op.str):])

		if property == "" {
			return nil, fmt.Errorf("missing property name in filter: %s", s)
		}

		f := &Filter{
			Property: property,
			Operator: op.op,
			Value:    value,
		}

		if err := finalizeFilter(f); err != nil {
			return nil, err
		}

		return f, nil
	}

	return nil, fmt.Errorf("invalid filter expression (missing operator): %s", s)
}

// finalizeFilter compiles regex patterns and validates glob patterns.
func finalizeFilter(f *Filter) error {
	// Compile regex if needed
	if f.Operator == OpRegex {
		re, err := regexp.Compile(f.Value)
		if err != nil {
			return fmt.Errorf("invalid regex pattern %q: %w", f.Value, err)
		}
		f.Regex = re
	}

	// Check for glob patterns in equality operator
	// Pre-compile the regex to avoid recompilation on every match
	if f.Operator == OpEqual && ContainsUnescapedGlob(f.Value) {
		if err := ValidatePattern(f.Value); err != nil {
			return fmt.Errorf("invalid glob pattern %q: %w", f.Value, err)
		}
		regexPattern := GlobToRegex(f.Value)
		re, err := regexp.Compile(regexPattern)
		if err != nil {
			return fmt.Errorf("failed to compile glob pattern %q: %w", f.Value, err)
		}
		f.Regex = re
		f.IsGlob = true
	}

	// Handle fuzzy operator with optional wildcard suffix
	if f.Operator == OpFuzzy {
		fuzzyPart, _, hasWildcard := SplitFuzzyWildcard(f.Value)
		f.FuzzyTarget = fuzzyPart

		if hasWildcard {
			// Validate and compile full pattern as glob
			if err := ValidatePattern(f.Value); err != nil {
				return fmt.Errorf("invalid fuzzy+wildcard pattern %q: %w", f.Value, err)
			}
			// Compile the FULL value as a glob regex
			// This combines the fuzzy prefix with wildcard suffix for final matching
			regexPattern := GlobToRegex(f.Value)
			re, err := regexp.Compile(regexPattern)
			if err != nil {
				return fmt.Errorf("failed to compile pattern %q: %w", f.Value, err)
			}
			f.WildcardRe = re
		}
	}

	return nil
}

// ParseAll parses multiple filter strings
func ParseAll(filters []string) ([]*Filter, error) {
	result := make([]*Filter, 0, len(filters))
	for _, s := range filters {
		f, err := Parse(s)
		if err != nil {
			return nil, err
		}
		result = append(result, f)
	}
	return result, nil
}

// MatchValue checks if a value matches the filter.
// For list values ([]string, []interface{}):
//   - For = operator: returns true if ANY element matches
//   - For != operator: returns true if NO element matches
func MatchValue(value interface{}, f *Filter) bool {
	// Handle list types
	switch v := value.(type) {
	case []string:
		return matchStringList(v, f)
	case []interface{}:
		strList := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				strList = append(strList, s)
			}
		}
		return matchStringList(strList, f)
	}

	// Convert scalar value to string for comparison
	var strValue string
	switch v := value.(type) {
	case string:
		strValue = v
	case int, int32, int64:
		strValue = fmt.Sprintf("%d", v)
	case float32, float64:
		strValue = fmt.Sprintf("%f", v)
	case bool:
		strValue = strconv.FormatBool(v)
	default:
		strValue = fmt.Sprintf("%v", v)
	}
	return matchStringSimple(strValue, f)
}

// matchStringList handles filter matching for string lists (multi-select values)
func matchStringList(list []string, f *Filter) bool {
	if len(list) == 0 {
		return matchStringSimple("", f)
	}

	// For != operator: return true only if NO element matches the value
	if f.Operator == OpNotEqual {
		for _, s := range list {
			if s == f.Value {
				return false // Found a match, so "not equal" is false
			}
		}
		return true // No element matched
	}

	// For all other operators: return true if ANY element matches
	for _, s := range list {
		if matchStringSimple(s, f) {
			return true
		}
	}
	return false
}

// matchStringSimple checks if a string value matches the filter (simple version without errors)
func matchStringSimple(strValue string, f *Filter) bool {
	switch f.Operator {
	case OpEqual:
		if f.IsGlob {
			// Use pre-compiled regex from finalizeFilter
			if f.Regex == nil {
				return false
			}
			return f.Regex.MatchString(strValue)
		}
		return strValue == f.Value

	case OpNotEqual:
		if f.IsGlob {
			// Use pre-compiled regex from finalizeFilter
			if f.Regex == nil {
				return false
			}
			return !f.Regex.MatchString(strValue)
		}
		return strValue != f.Value

	case OpLess:
		return strValue < f.Value

	case OpLessEqual:
		return strValue <= f.Value

	case OpGreater:
		return strValue > f.Value

	case OpGreaterEqual:
		return strValue >= f.Value

	case OpRegex:
		if f.Regex == nil {
			return false
		}
		return f.Regex.MatchString(strValue)

	case OpFuzzy:
		// Inline fuzzy matching logic to avoid error handling complexity
		target := f.FuzzyTarget
		if target == "" {
			return false
		}

		if f.WildcardRe != nil {
			// Two-phase match: glob pattern + fuzzy prefix
			if !f.WildcardRe.MatchString(strValue) {
				return false
			}
			targetRunes := []rune(target)
			valueRunes := []rune(strValue)
			if len(valueRunes) < len(targetRunes) {
				return false
			}
			prefix := string(valueRunes[:len(targetRunes)])
			return TrigramSimilarity(prefix, target) >= DefaultFuzzyThreshold
		}

		return TrigramSimilarity(strValue, target) >= DefaultFuzzyThreshold

	default:
		return false
	}
}
