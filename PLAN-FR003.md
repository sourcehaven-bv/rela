# Plan: FR-003 - Filter Entities by Custom Properties

## Overview

Implement generic property filtering for `rela list` with typed properties, comparison operators, and sorting support.

## Changes

### 1. Extend Metamodel Property Types

**File:** `internal/metamodel/types.go`

Add new built-in types and format field to `PropertyDef`:

```go
type PropertyDef struct {
    Type        string   `yaml:"type"`               // string, date, integer, boolean, enum, or custom
    Required    bool     `yaml:"required,omitempty"`
    Values      []string `yaml:"values,omitempty"`   // For inline enum
    Default     string   `yaml:"default,omitempty"`
    Description string   `yaml:"description,omitempty"`
    Format      string   `yaml:"format,omitempty"`   // NEW: date format (Go layout)
}
```

Built-in types:
- `string` - free-form text
- `date` - date with optional format (default: `2006-01-02`)
- `integer` - whole numbers
- `boolean` - `true` or `false`
- `enum` - inline enum with `values` array
- `<custom>` - reference to type in `types:` section

### 2. Add Property Validation by Type

**File:** `internal/metamodel/validation.go`

Extend `ValidateEntity` to validate typed properties:

```go
func (m *Metamodel) ValidatePropertyValue(propName string, propDef *PropertyDef, value interface{}) error {
    switch propDef.Type {
    case "string":
        if _, ok := value.(string); !ok {
            return fmt.Errorf("property %s must be a string", propName)
        }
    case "date":
        s, ok := value.(string)
        if !ok {
            return fmt.Errorf("property %s must be a date string", propName)
        }
        format := propDef.Format
        if format == "" {
            format = "2006-01-02"
        }
        if _, err := time.Parse(format, s); err != nil {
            return fmt.Errorf("invalid date %q for property %s (expected format: %s)", s, propName, format)
        }
    case "integer":
        // Handle both int and string representation from YAML
        switch v := value.(type) {
        case int, int64, float64:
            // OK
        case string:
            if _, err := strconv.Atoi(v); err != nil {
                return fmt.Errorf("invalid integer %q for property %s", v, propName)
            }
        default:
            return fmt.Errorf("property %s must be an integer", propName)
        }
    case "boolean":
        switch v := value.(type) {
        case bool:
            // OK
        case string:
            if v != "true" && v != "false" {
                return fmt.Errorf("property %s must be true or false, got %q", propName, v)
            }
        default:
            return fmt.Errorf("property %s must be a boolean", propName)
        }
    case "enum":
        // existing inline enum validation
    default:
        // existing custom type validation
    }
    return nil
}
```

### 3. Create Filter Parser

**File:** `internal/filter/filter.go` (new file)

```go
package filter

type Operator int

const (
    OpEqual Operator = iota      // =
    OpNotEqual                   // !=
    OpLess                       // <
    OpLessEqual                  // <=
    OpGreater                    // >
    OpGreaterEqual               // >=
    OpRegex                      // =~
)

type Filter struct {
    Property string
    Operator Operator
    Value    string
    Regex    *regexp.Regexp  // compiled if OpRegex
}

// Parse parses a filter string like "status=draft" or "valid_until<2025-02-01"
func Parse(s string) (*Filter, error)

// Supported operators by type:
// - string: =, !=, =~ (regex), glob patterns with *
// - enum/custom: =, !=
// - date: =, !=, <, <=, >, >=
// - integer: =, !=, <, <=, >, >=
// - boolean: =, !=
```

### 4. Create Filter Matcher

**File:** `internal/filter/match.go` (new file)

```go
// Match checks if an entity matches a filter against the metamodel
func Match(entity *model.Entity, filter *Filter, propDef *metamodel.PropertyDef) (bool, error)

// MatchAll checks if entity matches all filters (AND semantics)
func MatchAll(entity *model.Entity, filters []*Filter, entityDef *metamodel.EntityDef) (bool, error)
```

Matching logic:
- `string` with `=`: exact match or glob pattern (if contains `*`)
- `string` with `=~`: regex match
- `enum`: exact match only, error on invalid value
- `date`: parse both values with format, compare
- `integer`: parse both as int, compare
- `boolean`: parse both as bool, compare

### 5. Update List Command

**File:** `internal/cli/list.go`

Remove:
- `listStatus` variable
- `listPriority` variable
- `--status` flag
- `--priority` flag

Add:
```go
var (
    listWhere []string  // repeatable --where flags
    listSort  string    // --sort property
    listDesc  bool      // --desc for descending
)

func init() {
    listCmd.Flags().StringArrayVar(&listWhere, "where", nil, `Filter by property (e.g., --where "status=draft")`)
    listCmd.Flags().StringVar(&listSort, "sort", "", "Sort by property")
    listCmd.Flags().BoolVar(&listDesc, "desc", false, "Sort descending")
}
```

Updated RunE logic:
1. Parse all `--where` filters
2. Validate each filter against metamodel (property exists, operator valid for type)
3. Filter entities using `filter.MatchAll`
4. Sort entities by `--sort` property (type-aware)
5. Output results

### 6. Implement Type-Aware Sorting

**File:** `internal/filter/sort.go` (new file)

```go
// Sort sorts entities by a property with type-aware comparison
func Sort(entities []*model.Entity, propName string, propDef *metamodel.PropertyDef, descending bool)
```

Sorting behavior:
- `string`/`enum`: lexicographic (`sort.Strings`)
- `date`: parse and compare as `time.Time`
- `integer`: parse and compare as `int`
- `boolean`: `false` < `true`

### 7. Update Documentation

**File:** `docs/metamodel.md`

Add section on property types:

```markdown
### Property Types

| Type | Description | Operators | Example |
|------|-------------|-----------|---------|
| `string` | Free-form text | `=`, `!=`, `=~` (regex) | `title=~.*policy.*` |
| `date` | Date value | `=`, `!=`, `<`, `<=`, `>`, `>=` | `valid_until<2025-02-01` |
| `integer` | Whole number | `=`, `!=`, `<`, `<=`, `>`, `>=` | `risk_score>=5` |
| `boolean` | True or false | `=`, `!=` | `archived=false` |
| `enum` | Inline enum | `=`, `!=` | `status=draft` |

#### Date Format

Specify date format using Go layout strings:

```yaml
properties:
  valid_until:
    type: date
    format: "2006-01-02"  # YYYY-MM-DD (default)
```

Common formats:
- `2006-01-02` → YYYY-MM-DD (ISO 8601, default)
- `02-01-2006` → DD-MM-YYYY
- `01/02/2006` → MM/DD/YYYY
- `2 Jan 2006` → D Mon YYYY
```

Add section on filtering:

```markdown
## Filtering Entities

Filter entities by property values:

```bash
# Exact match
rela list control --where "status=implemented"

# Glob pattern (strings)
rela list control --where "iso27001=A.9.*"

# Regex (strings)
rela list control --where "title=~access.*policy"

# Comparison (dates, integers)
rela list evidence --where "valid_until<2025-02-01"
rela list risk --where "risk_score>=5"

# Multiple filters (AND)
rela list control --where "status=implemented" --where "applicability=applicable"
```

## Sorting

```bash
rela list control --sort iso27001
rela list evidence --sort valid_until --desc
```
```

## File Summary

| File | Action |
|------|--------|
| `internal/metamodel/types.go` | Add `Format`, `Description` to `PropertyDef` |
| `internal/metamodel/validation.go` | Add type-specific validation |
| `internal/filter/filter.go` | New: filter parser |
| `internal/filter/match.go` | New: filter matching |
| `internal/filter/sort.go` | New: type-aware sorting |
| `internal/cli/list.go` | Replace `--status`/`--priority` with `--where`, add `--sort`/`--desc` |
| `docs/metamodel.md` | Document property types, filtering, sorting |

## Testing

### Unit Tests

- `internal/filter/filter_test.go`: Parse various filter syntaxes
- `internal/filter/match_test.go`: Match logic for all types and operators
- `internal/filter/sort_test.go`: Sorting for all types
- `internal/metamodel/validation_test.go`: Type validation

### Integration Tests

```bash
# Setup test entities with typed properties
rela create evidence EV-001 --title "Audit report" --valid_until "2025-06-01"
rela create evidence EV-002 --title "Policy doc" --valid_until "2025-01-15"
rela create risk RISK-001 --title "Data breach" --risk_score 8
rela create risk RISK-002 --title "Minor issue" --risk_score 3

# Test filtering
rela list evidence --where "valid_until<2025-03-01"  # Should return EV-002
rela list risk --where "risk_score>=5"               # Should return RISK-001

# Test sorting
rela list evidence --sort valid_until                # EV-002, EV-001
rela list evidence --sort valid_until --desc         # EV-001, EV-002

# Test error cases
rela list control --where "typo=value"               # Error: unknown property
rela list evidence --where "valid_until=bad-date"    # Error: invalid date
rela list control --where "status>draft"             # Error: operator not supported
```

## Migration

Existing `--status` and `--priority` flags are removed. Users should migrate to:

```bash
# Old
rela list --status accepted

# New
rela list --where "status=accepted"
```

This is a breaking change. Document in release notes.
