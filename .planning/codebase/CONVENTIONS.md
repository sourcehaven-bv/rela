# Coding Conventions

**Analysis Date:** 2026-03-19

## Naming Patterns

**Files:**
- Package-level files use lowercase with underscores: `entity.go`, `content_test.go`, `validate_test.go`
- Test files: `*_test.go` suffix (e.g., `handlers_test.go`, `repository_test.go`)
- Fuzz tests: `*_fuzz_test.go` suffix (e.g., `relation_fuzz_test.go`)
- Build tag test files: `e2e_test.go` with `//go:build e2e` directive

**Packages:**
- Lowercase, no underscores: `cli`, `model`, `graph`, `markdown`, `dataentry`
- Descriptive names reflecting responsibility: `metamodel`, `dataentryconfig`, `workspace`

**Functions:**
- CamelCase with exported functions capitalized: `NewEntity()`, `GetString()`, `AddNode()`
- Test helper functions use prefix: `testMeta()`, `testGraph()`, `testConfig()`, `newHandlerTestApp()`, `newE2ETestApp()`
- Handler functions use verb-noun pattern: `handleIndex()`, `handleList()`, `handleCreate()`
- Unexported internal helpers with `lowercase`: `indexEntityProperties()`, `unindexEntityProperties()`, `valueToString()`

**Variables:**
- CamelCase throughout: `createTitle`, `listWhere`, `propertyIndex`
- Package-level flags use descriptive names: `createProperties`, `createBodyFile`
- Loop variables use single letters: `i`, `c` (child), `v` (value)
- Receiver variables are short: `e` for Entity, `g` for Graph

**Types:**
- Exported types capitalized: `Entity`, `Relation`, `Graph`, `Metamodel`
- Custom error types end with `Error`: `EntityNotFoundError`, `EntityTypeNotFoundError`, `RelationNotFoundError`, `ValidationError`
- Sentinel errors use `Err` prefix: `ErrNotFound`, `ErrAlreadyExists`, `ErrInvalidID`
- Interface types follow purpose: `Migration`, `FileType`

**Constants:**
- All caps with underscores for package constants
- Ignored magic numbers: 0, 1, 2, 3, 10, 100, 0644, 0755, 0o644, 0o755

## Code Style

**Formatting:**
- Tool: gofmt with goimports
- Import order: standard library, third-party (including github.com), local (github.com/Sourcehaven-BV/rela)
- Line length: 120 characters (enforced by lll linter)
- Indentation: tabs (tab-width: 4 in lll config)

**Linting:**
- Tool: golangci-lint v1.62.2 (configured in `.golangci.yml`)
- Key rules:
  - errcheck: Check all errors, allow explicit `_ = ` to ignore
  - exhaustive: Require exhaustive enum switch statements
  - gofmt: Code must pass gofmt with simplify=true
  - goimports: Imports must be sorted and grouped per configured order
  - errorlint: Enforce proper error wrapping with `errors.Is()` and `errors.As()`
  - gosec: Security checks (excludes G104, G204, G304, G306)

**Conventions enforced by linters:**
- Function length limit: 100 lines (funlen)
- Function statements: 60 statements max (funlen)
- Cognitive complexity: 30+ triggers warning (gocognit)
- Cyclomatic complexity: 35+ triggers warning (gocyclo)
- Nested if depth: 6+ levels triggers warning (nestif)
- Naked returns: Max 30 lines (nakedret)
- No blank identifiers (`_`) without purpose (dogsled)
- Duplication threshold: 150 lines (dupl) - test files excluded
- Error naming: Sentinel errors must start with `Err` (errname)

## Import Organization

**Order:**
1. Standard library (`fmt`, `time`, `sync`, etc.)
2. Third-party packages (`github.com/spf13/cobra`, `gopkg.in/yaml.v3`, etc.)
3. Local packages (`github.com/Sourcehaven-BV/rela/internal/...`)

**Path Aliases:**
- Local prefix configured: `github.com/Sourcehaven-BV/rela`
- Example: `import "github.com/Sourcehaven-BV/rela/internal/model"`

**Blank imports:**
- Only for side effects (e.g., test framework imports)
- Must be justified

## Error Handling

**Patterns:**
- Use sentinel errors for known error types: `ErrNotFound`, `ErrInvalidID` (from `internal/errors/`)
- Custom error types implement `Error()` and `Unwrap()` for error chains: see `EntityNotFoundError`, `ValidationError`
- Wrap errors with context: `fmt.Errorf("context: %w", err)`
- Check errors: `if err != nil { return err }` (explicit error propagation)
- Explicit error ignoring allowed: `_ = risky()` (checked by errcheck)
- Type-specific error checking: Use `errors.Is()` for sentinel errors, `errors.As()` for type assertions

**Error messages:**
- Start with lowercase in custom error types: `fmt.Sprintf("entity not found: %s", e.ID)`
- Include context: field names, values, operation being performed
- Validation errors include field name: `ValidationError{Field: "title", Message: "required"}`

## Logging

**Framework:** None by convention; use `fmt.Fprintf()` or `fmt.Println()` for CLI output

**Patterns:**
- CLI commands write to output writer passed through context
- Use `out` (output writer) for structured output (handled by cli/root.go)
- Error messages go to stderr via error returns (CLI framework handles display)
- Data entry handlers use HTML template rendering (no logging needed)

## Comments

**When to Comment:**
- Public functions must have doc comments starting with function name: `// GetString returns...`
- Complex logic needing explanation: `// Initialize map for this property if needed`
- Non-obvious algorithmic choices: `// Property index stores value counts for filtering`
- Unexported helper functions with specific purpose: `// unindexEntityProperties removes entity property values from the property index`
- Business logic constraints: `// This allows renderers to access both built-in fields and custom properties without special-case handling.`

**JSDoc/TSDoc:**
- Not used in Go code
- Go doc comments follow standard Go convention: line comments above declarations

**Comment style:**
- English, complete sentences preferred for non-obvious code
- Explain why, not what (code shows what)
- No TODO/FIXME in production code (use issue tracker)

## Function Design

**Size:**
- Target: under 100 lines (funlen enforces this)
- Exceptions: HTTP handlers and config validation may exceed with justification in `.golangci.yml`
- Complex dataentry handlers (200+ lines) allowed due to template building complexity

**Parameters:**
- Receiver: short, single letter (e, g, h)
- Arguments: descriptive names, function parameters avoid single letters except loop vars
- Return: error always last return value
- Variadic parameters preferred for options: not used heavily in this codebase

**Return Values:**
- Always return error last: `(result, error)`
- Single return value for simple getters: `e.Title() string`
- Multiple returns for operations: `(value, error)` or `(result, modified bool, error)`
- Use named returns sparingly; mostly for clarity in complex functions

## Module Design

**Exports:**
- Only public, stable APIs are exported (capitalized)
- Unexported functions keep implementation private
- Structs export field names capitalized: `Entity{ID, Type, Properties, Content, FilePath, ModTime}`
- Avoid exporting implementation details

**Barrel Files:**
- Not used in this codebase
- Each package is self-contained with direct imports

**Package initialization:**
- init() functions allowed (despite gochecknoinits being disabled for Cobra CLIs)
- Cobra commands register themselves via init() patterns in cli/create.go, cli/list.go, etc.

## Struct Design

**Field ordering:**
- Public fields before unexported
- Related fields grouped logically
- Example from `Entity`: ID, Type, Properties (public); Content, FilePath, ModTime (metadata)
- Example from `Graph`: nodes, edges, indices (private storage)

**Naming:**
- Struct field names: CamelCase, no prefix/suffix conventions
- Receiver method names describe action: `AddNode()`, `GetString()`, `GetAttribute()`

## Type Conversions

**Patterns:**
- Type assertions check second return: `if s, ok := v.(string) { ... }`
- Interface{} used for flexible property maps: `map[string]interface{}`
- Value coercion in helpers: `GetAttributeString()` coerces to string safely
- List coercion: `GetAttributeStrings()` handles []string and []interface{}

## Variable Scope

**Package-level:**
- Flags in CLI commands use package vars: `var createTitle string` (set by cobra.Command PreRun)
- Test fixtures as package vars: `var createCmd = &cobra.Command{...}`

**Local scope preferred:**
- Loop variables declared in for: `for i := 0; i < len(items); i++ { ... }`
- Named returns only when clarity needed
- Defer for cleanup: `defer g.mu.Unlock()` after lock

---

*Convention analysis: 2026-03-19*
