---
id: TKT-UG3C
type: ticket
title: Add appbuild.NewForTest fixture and migrate CLI tests off workspace.NewForTest
kind: refactor
priority: high
effort: m
status: backlog
---

Add a `NewForTest` fixture to `internal/appbuild` matching the shape CLI tests
need, then swap the four CLI test files off `workspace.NewForTest`.

**API:**

```go
func NewForTest(meta *metamodel.Metamodel, opts ...TestOption) *Services
```

**Design decisions:**

- Takes `*Metamodel` directly (bypasses the loader, so pre-migration test metamodels work; matches `workspace.NewForTest` behavior)
- Returns production `*Services` so tests use the same accessor surface
- Options:
  - `WithTestStore(store.Store)` — pre-built store for seeded fixtures
  - `WithFS(fs storage.FS, paths *project.Context)` — for paths-aware code
- **No `WithScript`** — CLI tests do not need to drive automation. Lua-cascade paths are covered at e2e/script level. Engine setup happens unconditionally; tests with empty automation sets pay the cheap construction cost
- No `Close()` in test cleanup — tests today don't bother with workspace cleanup either

**Files to migrate:**

- `internal/cli/test_helpers_test.go`
- `internal/cli/export_test.go`
- `internal/cli/validate_test.go`
- `internal/cli/rename_test.go` (uses pre-migration test metamodel; the `*Metamodel`-direct path matters here)

Also adds `internal/appbuild/appbuild_test.go` cases for `NewForTest` itself.

**Scope:** ~250 LOC.

See `.ignored/cli-off-workspace-plan.md` PR 3 for full detail.
