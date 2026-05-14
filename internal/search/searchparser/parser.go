package searchparser

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/Sourcehaven-BV/rela/internal/filter"
)

// SearchQuery represents parsed search query components
type SearchQuery struct {
	EntityTypes     []string          // Entity types to filter (e.g., ["requirement", "decision"])
	PropertyFilters []*filter.Filter  // Property filters (e.g., status=published)
	FreeTextWords   []string          // Free text words (OR logic with scoring)
	FreeTextPhrases []string          // Exact phrase matches (quoted strings)
	SortClauses     []filter.SortSpec // Sort criteria (e.g., sort:priority:desc)
	ParseErrors     []string          // Any parsing errors encountered
}

// ParseQuery parses a search query string into its components
// Example: "type:requirement prop:status=published authentication"
func ParseQuery(query string) *SearchQuery {
	sq := &SearchQuery{
		EntityTypes:     []string{},
		PropertyFilters: []*filter.Filter{},
		FreeTextWords:   []string{},
		FreeTextPhrases: []string{},
		SortClauses:     []filter.SortSpec{},
		ParseErrors:     []string{},
	}

	if query == "" {
		return sq
	}

	// Tokenize the query, preserving quoted strings
	tokens := tokenize(query)

	for _, token := range tokens {
		// Check for type filter
		if strings.HasPrefix(token, "type:") {
			typeStr := strings.TrimPrefix(token, "type:")
			if typeStr == "" {
				sq.ParseErrors = append(sq.ParseErrors, "type: filter requires a value")
				continue
			}
			// Support comma-separated types
			types := strings.Split(typeStr, ",")
			for _, t := range types {
				t = strings.TrimSpace(t)
				if t != "" {
					sq.EntityTypes = append(sq.EntityTypes, t)
				}
			}
			continue
		}

		// Check for property filter
		if strings.HasPrefix(token, "prop:") {
			propStr := strings.TrimPrefix(token, "prop:")
			if propStr == "" {
				sq.ParseErrors = append(sq.ParseErrors, "prop: filter requires a value")
				continue
			}
			// Parse property filter using existing filter package
			f, err := filter.Parse(propStr)
			if err != nil {
				sq.ParseErrors = append(sq.ParseErrors, fmt.Sprintf("invalid property filter '%s': %v", propStr, err))
				continue
			}
			sq.PropertyFilters = append(sq.PropertyFilters, f)
			continue
		}

		// Check for status shortcut (maps to prop:status=)
		if strings.HasPrefix(token, "status:") {
			statusStr := strings.TrimPrefix(token, "status:")
			if statusStr == "" {
				sq.ParseErrors = append(sq.ParseErrors, "status: filter requires a value")
				continue
			}
			// Convert to property filter
			f, err := filter.Parse("status=" + statusStr)
			if err != nil {
				sq.ParseErrors = append(sq.ParseErrors, fmt.Sprintf("invalid status filter: %v", err))
				continue
			}
			sq.PropertyFilters = append(sq.PropertyFilters, f)
			continue
		}

		// Check for sort clause
		if strings.HasPrefix(token, "sort:") {
			sortStr := strings.TrimPrefix(token, "sort:")
			if sortStr == "" {
				sq.ParseErrors = append(sq.ParseErrors, "sort: requires a property name")
				continue
			}
			parts := strings.SplitN(sortStr, ":", 2)
			property := parts[0]
			direction := "asc"
			if len(parts) == 2 {
				switch parts[1] {
				case "asc", "desc":
					direction = parts[1]
				default:
					sq.ParseErrors = append(sq.ParseErrors,
						fmt.Sprintf("sort: invalid direction %q (use \"asc\" or \"desc\")", parts[1]))
					continue
				}
			}
			sq.SortClauses = append(sq.SortClauses, filter.SortSpec{
				Property:  property,
				Direction: direction,
			})
			continue
		}

		// Check if it's a quoted phrase
		if strings.HasPrefix(token, "\"") && strings.HasSuffix(token, "\"") {
			phrase := strings.Trim(token, "\"")
			if phrase != "" {
				sq.FreeTextPhrases = append(sq.FreeTextPhrases, phrase)
			}
			continue
		}

		// Otherwise, it's a free text word
		if token != "" {
			sq.FreeTextWords = append(sq.FreeTextWords, token)
		}
	}

	return sq
}

// tokenize splits a query string into tokens, preserving quoted strings
func tokenize(query string) []string {
	var tokens []string
	var current strings.Builder
	inQuotes := false
	escaped := false

	for _, r := range query {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}

		if r == '\\' {
			escaped = true
			continue
		}

		if r == '"' {
			inQuotes = !inQuotes
			current.WriteRune(r)
			if !inQuotes {
				// End of quoted string
				tokens = append(tokens, current.String())
				current.Reset()
			}
			continue
		}

		if unicode.IsSpace(r) && !inQuotes {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			continue
		}

		current.WriteRune(r)
	}

	// Add final token
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

// HasFilters returns true if the query has any type or property filters
func (sq *SearchQuery) HasFilters() bool {
	return len(sq.EntityTypes) > 0 || len(sq.PropertyFilters) > 0
}

// HasFreeText returns true if the query has any free text search terms
func (sq *SearchQuery) HasFreeText() bool {
	return len(sq.FreeTextWords) > 0 || len(sq.FreeTextPhrases) > 0
}

// HasSort returns true if the query has any sort clauses
func (sq *SearchQuery) HasSort() bool {
	return len(sq.SortClauses) > 0
}

// IsEmpty returns true if the query has no search components.
// A query with only sort clauses is still considered empty (sort without results is meaningless).
func (sq *SearchQuery) IsEmpty() bool {
	return !sq.HasFilters() && !sq.HasFreeText()
}

// ErrorString returns all parse errors as a single string
func (sq *SearchQuery) ErrorString() string {
	if len(sq.ParseErrors) == 0 {
		return ""
	}
	return strings.Join(sq.ParseErrors, "; ")
}

// AutocompleteContext represents the context for autocomplete at cursor position
type AutocompleteContext struct {
	Type         string // "type", "prop", "propvalue", "status", or ""
	Prefix       string // The partial text after the colon or operator
	PropertyName string // For "propvalue", the property name being completed
}

// GetAutocompleteContext analyzes the query at cursor position for autocomplete
func GetAutocompleteContext(query string, cursorPos int) *AutocompleteContext {
	if cursorPos > len(query) {
		cursorPos = len(query)
	}

	// Get text up to cursor
	textToCursor := query[:cursorPos]

	// Find the last token before cursor
	// We need to handle spaces and find the current incomplete token
	lastSpace := strings.LastIndexAny(textToCursor, " \t")
	var currentToken string
	if lastSpace == -1 {
		currentToken = textToCursor
	} else {
		currentToken = textToCursor[lastSpace+1:]
	}

	// Check if we're in the middle of a type: filter
	if strings.HasPrefix(currentToken, "type:") {
		prefix := strings.TrimPrefix(currentToken, "type:")
		return &AutocompleteContext{
			Type:   "type",
			Prefix: prefix,
		}
	}

	// Check if we're in the middle of a prop: filter
	if strings.HasPrefix(currentToken, "prop:") {
		propPart := strings.TrimPrefix(currentToken, "prop:")
		// Check if there's an operator and we're typing the value
		for _, op := range []string{"!=", "<=", ">=", "=~", "=", "<", ">"} {
			if idx := strings.Index(propPart, op); idx > 0 {
				// Found operator - we're completing the value
				propName := propPart[:idx]
				valuePrefix := propPart[idx+len(op):]
				return &AutocompleteContext{
					Type:         "propvalue",
					PropertyName: propName,
					Prefix:       valuePrefix,
				}
			}
		}
		// No operator yet - completing property name
		return &AutocompleteContext{
			Type:   "prop",
			Prefix: propPart,
		}
	}

	// Check if we're in the middle of a status: filter
	if strings.HasPrefix(currentToken, "status:") {
		prefix := strings.TrimPrefix(currentToken, "status:")
		return &AutocompleteContext{
			Type:   "status",
			Prefix: prefix,
		}
	}

	// Check if we're in the middle of a sort: clause
	if strings.HasPrefix(currentToken, "sort:") {
		sortPart := strings.TrimPrefix(currentToken, "sort:")
		parts := strings.SplitN(sortPart, ":", 2)
		if len(parts) == 2 {
			// After property name, completing direction
			return &AutocompleteContext{
				Type:   "sortdir",
				Prefix: parts[1],
			}
		}
		// Completing property name
		return &AutocompleteContext{
			Type:   "sort",
			Prefix: parts[0],
		}
	}

	return &AutocompleteContext{Type: "", Prefix: ""}
}
