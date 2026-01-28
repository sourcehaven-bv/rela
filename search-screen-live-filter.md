# Design: Live Search Screen with Advanced Filtering

**Status:** Draft
**Created:** 2026-01-27
**Component:** TUI Search Screen

## Overview

Enhance the search screen with live search (no need to press Enter) and advanced filtering syntax
to enable powerful entity discovery and filtering.

## Goals

1. **Live Search**: Search updates as you type using goroutines for responsiveness
2. **Advanced Filter Syntax**: Support entity type, property, and free-text filtering
3. **Global Shortcut**: Quick access via `/` key from any screen
4. **Performance**: Handle large entity sets (1000+) without blocking UI

## User Experience

### Current Behavior

- Press `/` to open search screen
- Type query and press `Enter` to search
- Simple text search in ID, title, description, content
- Navigate results with `j`/`k`, open with `Enter`

### Proposed Behavior

- Press `/` to open search screen (✓ already implemented in `tui.go:137-140`)
- Type query → **search updates automatically** (no Enter required)
- Support advanced filter syntax (see below)
- Debounce search by ~200ms to avoid excessive updates
- Show live result count as you type
- Highlight filter parse errors inline

## Filter Syntax

### Syntax Components

```text
[type_filter] [free_text] [property_filter]... [special_filter]...
```

All components are **space-separated** and can appear in any order.

### 1. Entity Type Filter

**Prefix:** `type:`

```text
type:requirement           # only requirements
type:decision,solution     # multiple types (comma-separated)
```

### 2. Free Text Search

**No prefix** - plain words search across ID, title, description, and content.

**Logic:** Multiple words use **AND** - all words must be present.

```text
authentication api         # entities containing both "authentication" AND "api"
"multi word phrase"        # exact phrase match (quoted strings)
auth "REST API"            # contains "auth" AND exact phrase "REST API"
```

### 3. Property Filters

**Prefix:** `prop:`

Uses existing `internal/filter` package operators:

```text
prop:state=published       # exact match
prop:state!=draft          # not equal
prop:priority>3            # greater than
prop:priority>=3           # greater or equal
prop:review_at<2026-02-01  # less than (dates/strings)
prop:review_at<=2026-02-01 # less or equal
prop:title=*auth*          # glob pattern (existing support)
prop:desc=~^[A-Z].*        # regex match (existing support)
```

Multiple property filters are combined with AND logic:

```text
prop:state=published prop:priority>=2
```

### 4. Special Filters (Future)

**Status shortcut:** `status:` maps to `prop:status=`

```text
status:draft               # equivalent to prop:status=draft
status:published           # equivalent to prop:status=published
```

**Tag search (future):** `@tag` searches for tags in properties

```text
@security                  # entities tagged with "security"
```

### Complete Examples

```bash
# Find published requirements about authentication
type:requirement prop:state=published authentication

# High priority drafts across all types
prop:priority>3 prop:state=draft

# Decisions that need review before Feb 1, 2026
type:decision prop:review_at<2026-02-01

# Solutions containing "API" not in draft status
type:solution "REST API" prop:state!=draft

# Multiple entity types with property filter
type:requirement,decision prop:priority>=2 urgent

# Exact phrase with free text
authentication "OAuth 2.0" security
```

### Parse and Validation

1. **Split by spaces** (preserving quoted strings: `"exact phrase"`)
2. **Classify each token:**
   - `type:` → entity type filter
   - `prop:` → property filter (parse with `filter.Parse`)
   - `"quoted string"` → exact phrase search
   - Otherwise → free text word
3. **Validate property filters:** Use `filter.Parse` to validate syntax
4. **Autocomplete suggestions:**
   - After typing `type:` → show available entity types from metamodel
   - After typing `prop:` → show available property names
   - After typing property name and operator → suggest values if enumerated
5. **Show inline errors:** Display parse errors below the search box

## Technical Architecture

### Concurrency Model

```text
User Input → Debouncer (200ms) → Search Goroutine → Results Channel → Update UI
```

**Key Components:**

1. **Input Handler**: Captures keystrokes, updates query string
2. **Debouncer**: Waits 200ms after last keystroke before triggering search
3. **Search Worker**: Runs in goroutine, sends results via channel
4. **Message Handler**: Receives results and updates UI

### Data Flow

```go
// Message types
type searchQueryMsg struct {
    query string
}

type searchResultsMsg struct {
    results []*model.Entity
    query   string  // which query produced these results
    err     error
}

// Search command (debounced)
func (s *SearchModel) searchCmd(query string) tea.Cmd {
    return func() tea.Msg {
        time.Sleep(200 * time.Millisecond)  // debounce
        // Check if query changed (cancel search if stale)

        results, err := s.performSearch(query)
        return searchResultsMsg{
            results: results,
            query:   query,
            err:     err,
        }
    }
}
```

### Search Algorithm

```go
func (s *SearchModel) performSearch(query string) ([]*model.Entity, error) {
    // 1. Parse query into components
    parts := parseQuery(query)

    // 2. Get all entities from graph
    allEntities := s.app.graph.AllNodes()

    // 3. Filter by entity type (if specified)
    if parts.entityTypes != nil {
        allEntities = filterByType(allEntities, parts.entityTypes)
    }

    // 4. Apply property filters
    for _, propFilter := range parts.propertyFilters {
        allEntities = applyPropertyFilter(allEntities, propFilter)
    }

    // 5. Apply free-text search
    if parts.freeText != "" {
        allEntities = filterByFreeText(allEntities, parts.freeText)
    }

    // 6. Return all results (no limit - lazy loading in UI)
    return allEntities, nil
}
```

### Property Filter Integration

Leverage existing `internal/filter` package:

```go
// Parse property filter string "state=published"
propFilter, err := filter.Parse("state=published")
if err != nil {
    return err  // show parse error
}

// Apply filter to entities
filtered := []
for _, entity := range entities {
    value, exists := entity.Properties[propFilter.Property]
    if !exists {
        continue
    }
    if matchesFilter(value, propFilter) {
        filtered = append(filtered, entity)
    }
}
```

**Note:** Need to add `MatchValue(value string, filter *Filter) bool` helper to `internal/filter` package.

## UI Components

### Search Input Box

```text
┌─────────────────────────────────────────────────────┐
│ type:requirement prop:state=published auth_         │
└─────────────────────────────────────────────────────┘
Type to search (live), ↑/↓ to navigate, Enter to open
```

With syntax error:

```text
┌─────────────────────────────────────────────────────┐
│ prop:state published auth_                          │
└─────────────────────────────────────────────────────┘
⚠ Invalid property filter syntax: missing operator
```

### Results Display

Results are **lazily rendered** - only visible items are rendered, allowing infinite scrolling through large result sets.

```text
Found 1,523 results:

►  REQ-001      Authentication Requirements    (requirement)
   DEC-042      API Security Decision          (decision)
   SOL-123      OAuth Implementation           (solution)
   REQ-055      User Login Flow                (requirement)
   ...

[4/1523] Showing 1-20
```

Scroll with `j`/`k` or arrow keys. Results load dynamically as you scroll.

### Status Bar

The `/` shortcut is already implemented in `internal/tui/tui.go:137-140`:

```go
case "/":
    if a.screen != ScreenSearch {
        return a, a.pushScreen(ScreenSearch)
    }
```

Status bar should show:

```text
[/] search  [c] create  [g] graph  [a] analyze  [q] quit  [?] help
```

## Implementation Plan

### Phase 1: Live Search Foundation

1. **Add goroutine-based search**
   - Create `searchQueryMsg` and `searchResultsMsg` types
   - Implement `searchCmd` that runs in background
   - Add debouncing (200ms)
   - Handle concurrent searches (cancel stale results)

2. **Remove Enter requirement**
   - Trigger search on every keystroke (debounced)
   - Keep Enter to open selected result
   - Update `search.go:Update` to remove search on Enter

3. **Add search cancellation**
   - Track current search ID/version
   - Ignore results from stale searches
   - Cancel in-flight searches when new query arrives

### Phase 2: Filter Syntax Parser

1. **Create `internal/tui/searchparser` package**
   - `ParseQuery(query string) (*SearchQuery, error)`
   - `SearchQuery` struct with `EntityTypes`, `PropertyFilters`, `FreeText`
   - Token classification logic

2. **Integrate with existing `internal/filter`**
   - Use `filter.Parse` for property filter parsing
   - Add `filter.MatchValue` helper function
   - Support all existing operators (=, !=, <, <=, >, >=, =~)

3. **Entity type filtering**
   - Parse `type:` and `t:` prefixes
   - Support comma-separated types
   - Validate against metamodel types

### Phase 3: Search Implementation

1. **Update `SearchModel.performSearch`**
   - Parse query with new parser
   - Apply filters in order: type → properties → free text
   - Optimize for large entity sets

2. **Add property filter matching**
   - Implement `matchesPropertyFilter(entity, filter)`
   - Handle missing properties gracefully
   - Support type-aware comparisons (string, int, date)

3. **Improve free-text search**
   - Split into words (preserving quoted phrases)
   - Require all words/phrases present (AND logic)
   - Case-insensitive matching
   - Support quoted exact phrases: `"REST API"`

### Phase 4: UI Enhancements

1. **Syntax highlighting in input**
   - Color-code filter types (type: blue, prop: green, text: white)
   - Highlight quoted strings in yellow
   - Show parse errors inline with red color

2. **Autocomplete suggestions**
   - Show suggestions as a simple list below input field (not a dropdown UI component)
   - Navigate suggestions with ↑/↓ arrow keys (shifts selector)
   - Accept suggestion with Tab or Enter
   - Context-aware:
     - After `type:` → list entity types from metamodel
     - After `prop:` → list property names from metamodel
     - After `prop:name=` → suggest values if enum type
   - Display format:

     ```text
     ┌─────────────────────────────────────┐
     │ type:_                              │
     └─────────────────────────────────────┘

     Suggestions:
     ►  requirement
        decision
        solution
        component
     ```

3. **Search statistics**
   - Show "Searching..." indicator during search
   - Display result count: "Found 15 results"
   - Show which filters are active

4. **Help text**
   - Add quick syntax reference in search screen
   - Show examples on empty search
   - Add to main help screen

### Autocomplete Implementation Details

**Simplified approach** - no dropdown component needed:

1. **Parse partial query** up to cursor position
2. **Determine context**: Are we completing `type:`, `prop:`, or a property value?
3. **Show suggestions as a list** below input (similar to results list)
4. **Two modes**:
   - **Search mode** (default): ↑/↓ navigate results, suggestions hidden
   - **Autocomplete mode** (when typing after `type:` or `prop:`): ↑/↓ navigate suggestions
5. **Accept with Tab or Enter**: Insert suggestion at cursor, return to search mode

**State management:**

```go
type SearchModel struct {
    query           string
    cursorPos       int
    results         []*model.Entity
    resultIndex     int

    // Autocomplete state
    suggestions     []string
    suggestionIndex int
    inAutocomplete  bool  // true when showing suggestions
}
```

**When to show suggestions:**

- User types `type:` → show entity types from metamodel
- User types `prop:` → show property names from metamodel
- User types `prop:priority=` → show enum values (if applicable)
- User presses Escape → hide suggestions, return to search mode

**This is simple to implement** - just render a list like we already do for results! No complex dropdown UI component required.

## Testing Strategy

### Unit Tests

1. **Query Parser Tests** (`searchparser_test.go`)
   - Parse various query formats
   - Handle syntax errors gracefully
   - Edge cases: empty, malformed, special chars

2. **Filter Application Tests**
   - Entity type filtering
   - Property filtering with all operators
   - Free-text search
   - Combined filters

3. **Concurrency Tests**
   - Multiple rapid searches
   - Search cancellation
   - Stale result handling

### Integration Tests

1. **TUI Tests** (if framework supports)
   - Type query → verify live update
   - Test debouncing behavior
   - Navigate and open results

### Performance Tests

1. **Benchmark searches**
   - 100 entities: < 10ms
   - 1000 entities: < 50ms
   - 10000 entities: < 200ms

2. **Memory profiling**
   - No goroutine leaks
   - Bounded result sets

## Configuration

Add to project settings (future):

```yaml
# .rela/config.yaml
search:
  debounce_ms: 200        # debounce delay
  max_results: 100        # result limit
  live_search: true       # enable/disable live search
```

## Documentation Updates

1. **User Guide**
   - Add "Search and Filtering" section
   - Document all filter syntax
   - Provide examples

2. **CLAUDE.md**
   - Update TUI architecture section
   - Document search concurrency model

3. **Help Screen**
   - Add filter syntax quick reference
   - Link to full documentation

## Future Enhancements

1. **Saved Searches**
   - Save frequent queries
   - Keyboard shortcuts for saved searches

2. **Search History**
   - Recent searches with ↑/↓ in empty search box
   - Clear history command

3. **Fuzzy Matching**
   - Approximate string matching for typos
   - Use library like `github.com/sahilm/fuzzy`

4. **Relation Filtering**
   - `rel:depends-on` - entities with specific relations
   - `rel:depends-on:REQ-001` - entities linked to specific entity

5. **Date Parsing**
   - Support relative dates: `prop:review_at<today+7d`
   - Natural language: `prop:review_at<next-week`

6. **Search Result Actions**
   - Bulk operations on search results
   - Export search results

7. **Visual Query Builder**
   - Interactive filter builder (alternative to syntax)
   - Form-based filtering

## Decisions Made

1. ✅ **Prefixes**: Use long form only (`type:`, `prop:`)
2. ✅ **Free Text Logic**: AND - all words must be present
3. ✅ **Result Rendering**: Lazy loading - no limit, render only visible items
4. ✅ **Quoted Strings**: Support `"exact phrase"` matching
5. ✅ **Autocomplete**: Show suggestions for types, properties, and values
6. ✅ **Debounce Timing**: 200ms (can make configurable later if needed)

## References

- Existing search: `internal/tui/search.go`
- Filter package: `internal/filter/filter.go`
- TUI architecture: `internal/tui/tui.go`
- Graph API: `internal/graph/graph.go`

---

**Next Steps:**

1. Discuss and finalize filter syntax
2. Get approval on implementation phases
3. Create task tickets for each phase
4. Begin Phase 1 implementation
