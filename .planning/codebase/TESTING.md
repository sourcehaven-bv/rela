# Testing Patterns

**Analysis Date:** 2026-03-19

## Test Framework

**Runner:**
- Go's built-in `testing` package (no external framework)
- Config: None required (standard Go testing)
- Supports build tags for optional tests: `//go:build e2e`

**Assertion Library:**
- None; manual assertions with `if` statements and `t.Errorf()`
- Example: `if w.Code != http.StatusOK { t.Errorf("expected 200, got %d", w.Code) }`

**Run Commands:**
```bash
just test                     # Run all tests with race detection
just test-verbose             # Run tests with verbose output
just test-coverage            # Generate coverage.out
just coverage-check           # Verify coverage meets thresholds
go test ./...                 # Quick test
go test -race ./...           # With race detection
go test -v ./...              # Verbose
go test -tags=e2e ./...       # Run tagged (e2e) tests only
just fuzz                     # Fuzz tests (30s each)
just fuzz-short               # Quick fuzz (5s each)
```

## Test File Organization

**Location:**
- Co-located with source: `handlers.go` → `handlers_test.go` (same package)
- Same package as code under test (no _test package suffix)

**Naming:**
- Test functions: `Test<FunctionName>` (e.g., `TestHandleIndex`, `TestExtractHeaders`)
- Test subtests: `t.Run("descriptor", func(t *testing.T) { ... })`
- Test helpers: `test<Resource>()` prefix (e.g., `testMeta()`, `testGraph()`)
- E2E tests: `TestE2E_<Description>` with `//go:build e2e` tag
- Fuzz tests: `Fuzz<FunctionName>` (e.g., `FuzzParseDocument`)

**Structure:**
```
internal/
  model/
    entity.go
    entity_test.go
    entity_fuzz_test.go          # Fuzz tests, //go:build fuzz (implicit)
  dataentry/
    handlers.go
    handlers_test.go
    e2e_test.go                  # E2E tests, //go:build e2e
```

## Test Structure

**Suite Organization:**
```go
func TestHandleIndex(t *testing.T) {
	t.Run("redirects to first list", func(t *testing.T) {
		// Arrange
		app := newHandlerTestApp(t)
		r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		w := httptest.NewRecorder()

		// Act
		app.handleIndex(w, r)

		// Assert
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
	})

	t.Run("non-root path returns 404", func(t *testing.T) {
		// Similar structure
	})
}
```

**Patterns:**
- Setup: Create test fixtures and prepare inputs (via helper like `newHandlerTestApp(t)`)
- Arrange-Act-Assert structure within each subtest
- Subtests allow running multiple cases in one test function
- Test helpers marked with `t.Helper()` to report errors at call site

**Setup pattern:**
```go
// newHandlerTestApp builds a full App for handler tests.
func newHandlerTestApp(t *testing.T) *App {
	t.Helper()  // Mark as helper so errors report at caller line
	meta := testMeta()
	cfg := testConfig()
	g := testGraph()
	// ... build and return *App
}

// testMeta returns a metamodel suitable for testing
func testMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Types: map[string]metamodel.CustomType{ ... },
		Entities: map[string]metamodel.EntityDef{ ... },
		Relations: map[string]metamodel.RelationDef{ ... },
	}
}
```

**Teardown pattern:**
- Most tests don't need cleanup (no files, no servers)
- E2E tests use `t.TempDir()` for automatic cleanup: `tmpDir := t.TempDir()`
- Server tests use `defer server.Close()` after `httptest.NewServer()`
- Explicit cleanup functions returned from helpers: `app, projectDir, cleanup := newE2ETestApp(t); defer cleanup()`

**Assertion pattern:**
```go
// Simple condition check
if w.Code != http.StatusOK {
	t.Errorf("expected 200, got %d", w.Code)
}

// String contains check
body := w.Body.String()
if !strings.Contains(body, "TKT-001") {
	t.Error("expected TKT-001 in list")
}

// HTML element checking (for HTTP handlers)
if !htmlHasElement(htmlStr, "form", map[string]string{"id": "edit-form"}) {
	t.Error("expected form with id=edit-form")
}
```

## Mocking

**Framework:** No external mocking library; use test doubles and dependency injection

**Patterns:**
```go
// Option 1: Interface satisfaction (preferred for graph operations)
type testGraph struct {
	nodes map[string]*model.Entity
	// ... minimal fields for test
}
func (g *testGraph) GetNode(id string) *model.Entity { return g.nodes[id] }

// Option 2: Struct replacement (for full app testing)
// Pass test struct instead of production struct
app := &App{
	Cfg: testConfig(),
	meta: testMeta(),
	g: testGraph(),
	// ... other fields
}

// Option 3: httptest.NewRequest/NewRecorder (for HTTP handlers)
r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
w := httptest.NewRecorder()
handler(w, r)  // Call handler, capture response

// Option 4: Memory filesystem (for file I/O tests)
fs := storage.NewMemFS()
// Write test files to fs, pass to code under test
```

**What to Mock:**
- HTTP requests/responses: Use `httptest.NewRequest()` and `httptest.NewRecorder()`
- File I/O: Use `storage.NewMemFS()` (memory filesystem)
- Graphs and metamodels: Build minimal test instances with `testMeta()`, `testGraph()`
- Workspace: Build via `workspace.NewWithGraph()` for isolated unit tests

**What NOT to Mock:**
- Business logic classes (Entity, Relation, Graph) - test real implementations
- Error types - test actual error returns
- Metamodel parsing - test with real YAML fixtures
- Handler execution - test full handler chain
- Template rendering - execute real templates in handler tests

## Fixtures and Factories

**Test Data:**
```go
// Table-driven test with inline data
tests := []struct {
	name    string
	content string
	want    []string
}{
	{
		name:    "no headers",
		content: "Just some text\nwithout headers",
		want:    nil,
	},
	{
		name:    "single header",
		content: "# Title\nSome content",
		want:    []string{"# Title"},
	},
	// ... more cases
}

for _, tt := range tests {
	t.Run(tt.name, func(t *testing.T) {
		got := ExtractHeaders(tt.content)
		if len(got) != len(tt.want) {
			t.Errorf("got %d headers, want %d", len(got), len(tt.want))
		}
	})
}

// Builder pattern for complex objects
func testMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Types: map[string]metamodel.CustomType{
			"status_type": {Values: []string{"open", "in_progress", "closed"}},
		},
		Entities: map[string]metamodel.EntityDef{ /* ... */ },
	}
}

// Entity builder for graph tests
e1 := model.NewEntity("TKT-001", "ticket")
e1.SetString("title", "First Ticket")
e1.SetString("status", "open")
g.AddNode(e1)
```

**Location:**
- Test helpers in same file (`*_test.go`)
- Shared fixtures in `app_test.go` for package-level helpers
- Domain objects (testMeta, testGraph) defined as helper functions, not constants

**Naming convention:**
- Builder functions: `test<Resource>()` (e.g., `testMeta()`, `testConfig()`, `testGraph()`)
- Setup helpers: `new<Context>TestApp()` (e.g., `newHandlerTestApp()`, `newE2ETestApp()`)

## Coverage

**Requirements:**
- Minimum thresholds per package enforced by `.testcoverage.yml` with coverage ratchet
- Core packages (model, errors): 95%
- Critical functionality (output, project, markdown): 85-90%
- Complex logic (graph, metamodel): 65-75%
- UI code (dataentry): 60% (lower due to template rendering complexity)
- Overall floor: 45%
- Coverage can never decrease (ratchet baseline in `.coverage-baseline`)

**Coverage-ignore annotations:**
```go
// coverage-ignore: main function - entry point, tested via integration tests
func main() {
	cli.Execute()
}

// coverage-ignore: requires external graphviz installation
func renderWithGraphviz() error {
	// ...
}
```

Valid reasons for `coverage-ignore`:
- Entry points (main, init)
- External tool dependencies (graphviz, chromium)
- OS-specific code that can't be mocked
- Unreasonable to unit test code (tested via integration tests instead)

**View Coverage:**
```bash
just coverage-html          # Generate coverage.html in browser
just coverage               # Print coverage.out to console
go tool cover -func=coverage.out  # Per-function coverage
```

## Test Types

**Unit Tests:**
- Scope: Single function or small component
- Approach: Isolated, fast, no I/O
- Example: `TestExtractHeaders()` tests markdown parsing
- Example: `TestPropertyContains()` tests data transformation
- Setup: Minimal fixtures (table-driven test data)
- Run: `go test ./internal/markdown/`

**Integration Tests:**
- Scope: Multiple components (handlers + templates + graph)
- Approach: Full App with test fixtures, HTTP request/response
- Example: `TestHandleList()` tests HTTP handler, template rendering, graph data
- Example: `TestHandleIndex()` tests navigation, routing, redirect
- Setup: `newHandlerTestApp(t)` builds full App with mocked filesystem
- Run: `go test ./internal/dataentry/`

**E2E Tests:**
- Framework: chromedp (headless Chrome automation)
- Scope: Full application from user perspective
- Approach: Real HTTP server, real project files (copied to temp), real browser
- Example: `TestE2E_MarkdownEditorSave()` tests form submission, file I/O, DOM state
- Build tag: `//go:build e2e` (run with `go test -tags=e2e`)
- Setup: `newE2ETestApp(t)` copies prototype project to temp dir, starts server
- Run: `just e2e` (or `go test -tags=e2e ./internal/dataentry/`)

**Fuzz Tests:**
- Scope: Robustness of parsing/validation
- Approach: Go's native fuzzing (Go 1.18+)
- Examples: `FuzzParseDocument()`, `FuzzParseEntityID()`, `FuzzValidateID()` in internal/markdown/, internal/model/
- Run: `just fuzz` (30 seconds each), `just fuzz-short` (5 seconds each)
- Crash corpus stored in testdata/ directories

## Common Patterns

**Async Testing:**
```go
// Not common in this codebase (mostly synchronous CLI/handler code)
// Goroutines used in graph operations under test:
// - Graph.AddNode() is thread-safe (uses mu.Lock/Unlock)
// - Test calls happen sequentially, but code is race-checked
```

**Error Testing:**
```go
// Expected error from validation
if err := repository.ValidateEntity(entity); err == nil {
	t.Error("expected validation error")
}
if !errors.Is(err, errors.ErrValidation) {
	t.Errorf("expected ErrValidation, got %v", err)
}

// Specific error type
var validateErr *errors.ValidationError
if errors.As(err, &validateErr); validateErr == nil {
	t.Error("expected ValidationError type")
}
if validateErr.Field != "title" {
	t.Errorf("expected field=title, got %s", validateErr.Field)
}

// Table-driven error tests
tests := []struct {
	name    string
	input   string
	wantErr bool
	errType error  // Sentinel error to check with errors.Is()
}{
	{"valid input", "TKT-001", false, nil},
	{"invalid input", "invalid", true, model.ErrInvalidID},
}
for _, tt := range tests {
	t.Run(tt.name, func(t *testing.T) {
		err := ValidateID(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidateID() error = %v, wantErr %v", err, tt.wantErr)
		}
		if tt.errType != nil && !errors.Is(err, tt.errType) {
			t.Errorf("ValidateID() error = %v, want %v", err, tt.errType)
		}
	})
}
```

**Subtest organization:**
```go
func TestHandler(t *testing.T) {
	// Multiple scenarios testing same handler
	t.Run("success case", func(t *testing.T) { /* ... */ })
	t.Run("not found case", func(t *testing.T) { /* ... */ })
	t.Run("validation error", func(t *testing.T) { /* ... */ })

	// Arrange once, multiple assertions (rare)
	app := newHandlerTestApp(t)

	t.Run("scenario A", func(t *testing.T) {
		// Can share app but tests are independent
	})
}
```

## Race Detection

**Always enabled in CI:**
```bash
go test -race ./...  # Detects concurrent access to shared memory
```

**Graph package is thread-safe:**
- All public methods lock `mu` before accessing shared state
- Tests exercise concurrency without explicit goroutines (the code creates them)

## Pre-commit Hook

**Location:** `.git/hooks/pre-commit` (installed via `just install-hooks`)

**Runs:**
- `just lint` - golangci-lint
- `just test` - go test with race detection

**Failure:** Blocks commit if lint or tests fail

**Install:** `just install-hooks`

---

*Testing analysis: 2026-03-19*
