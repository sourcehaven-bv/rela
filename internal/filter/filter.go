package filter

import (
	"fmt"
	"regexp"
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
	default:
		return "?"
	}
}

// Filter represents a parsed filter expression
type Filter struct {
	Property string
	Operator Operator
	Value    string
	Regex    *regexp.Regexp // Compiled regex if Operator is OpRegex
	IsGlob   bool           // True if Value contains glob patterns (for string equality)
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
		return nil, fmt.Errorf("empty filter expression")
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
		{"<", OpLess},
		{">", OpGreater},
		{"=", OpEqual},
	}

	for _, op := range operators {
		idx := strings.Index(s, op.str)
		if idx > 0 {
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

			// Compile regex if needed
			if op.op == OpRegex {
				re, err := regexp.Compile(value)
				if err != nil {
					return nil, fmt.Errorf("invalid regex pattern %q: %w", value, err)
				}
				f.Regex = re
			}

			// Check for glob patterns in equality operator
			if op.op == OpEqual && strings.Contains(value, "*") {
				f.IsGlob = true
			}

			return f, nil
		}
	}

	return nil, fmt.Errorf("invalid filter expression (missing operator): %s", s)
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

// GlobToRegex converts a glob pattern to a regex pattern
func GlobToRegex(glob string) string {
	var result strings.Builder
	result.WriteString("^")

	for i := 0; i < len(glob); i++ {
		c := glob[i]
		switch c {
		case '*':
			result.WriteString(".*")
		case '?':
			result.WriteString(".")
		case '.', '+', '(', ')', '[', ']', '{', '}', '^', '$', '|', '\\':
			result.WriteByte('\\')
			result.WriteByte(c)
		default:
			result.WriteByte(c)
		}
	}

	result.WriteString("$")
	return result.String()
}
