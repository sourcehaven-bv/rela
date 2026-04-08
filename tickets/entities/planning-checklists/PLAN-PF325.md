---
id: PLAN-PF325
type: planning-checklist
title: 'Planning: repo.Sync returns a fresh graph; Reload publishes atomically'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** Defined in TKT-Z7HL body. Change `repo.Sync` to return a fresh
`*graph.Graph`, fold the graph into `workspaceState`, publish atomically from
Reload/Sync, delete `graph.Clear()`, extend the concurrent reload race test to
iterate the graph in readers.

**Acceptance Criteria:** As listed in ticket body (7 items). All verified below.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:** N/A — this is internal Go concurrency refactor using
`sync/atomic.Pointer`, which was already established in TKT-252Y (#342).
Reference pattern: Go stdlib `atomic.Pointer[T]` publish/load for lock-free
read-mostly state.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Change `Repository.Sync(meta) (*graph.Graph,
*SyncResult, error)`. Internally build a fresh `graph.New()`, populate it,
return it. Fold the graph reference into `workspaceState` so a single
`atomic.Pointer.Store` publishes meta+automation+searchIdx+graph coherently.
Readers holding a pre-reload snapshot keep the old graph (never mutated). Delete
`graph.Clear()` and `rebuildAdjacency()` since no callers remain.

**Files modified (all already landed in #351):**
- `internal/repository/repository.go` — new `Sync` signature
- `internal/workspace/workspace.go` — `workspaceState.graph`, `Reload`, `Sync`, `New` all use fresh graph
- `internal/workspace/workspace_test.go` — `TestConcurrentReloadStateSnapshot` extended to iterate graph in readers
- `internal/graph/graph.go` — `Clear()` deleted
- `internal/repository/repository_test.go` — updated call sites
- `internal/dataentry/watcher_test.go` — updated call sites

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** N/A — internal concurrency refactor with no new
input surface.

**Security-Sensitive Operations:** None. Behavior-neutral for callers.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
1. Sync returns fresh graph → verified in `repository_test.go:287` (non-nil `g` returned).
2. Reload publishes atomically → `TestConcurrentReloadStateSnapshot` iterates the graph in reader goroutines and asserts `len(nodes) > 0` throughout concurrent reloads.
3. Sync on failure leaves old graph in place → exercised via workspace_test negative paths; old state survives if `Sync` returns error (new state only published on success).
4. `graph.Clear()` deletion → grep confirms no remaining callers.

**Edge Cases:**
- Concurrent readers + reloader: covered by extended race test.
- Concurrent writers + reloader: `writeMu` serialization (production pattern) documented in test preamble. Pre-existing entity-level race is out of scope (tracked separately).

**Negative Tests:** Sync error paths: old state remains if Sync fails.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** None realized. The change landed cleanly as part of TKT-PNPI (#351)
because the Tx primitive required the fresh-graph semantics anyway — the two
refactors were architecturally inseparable. No regressions in `go test -race
./...`.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] N/A — Internal concurrency refactor, no user-facing docs needed.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-PA4Y (critical, addressed) and RR-FJH7 (critical,
addressed) — both are the *driving* findings for this ticket and are now
resolved. See resolution fields on those entities.

## Note on delivery

This ticket's scope was delivered inline as part of TKT-PNPI (PR #351) because
the Tx primitive required fresh-graph semantics on reload — decoupling them
would have created transient intermediate states that made neither PR safely
mergeable. All 7 acceptance criteria verified after the fact:

1. `repo.Sync` returns fresh `*graph.Graph` — `repository.go:267`.
2. `Workspace.Reload` publishes via `workspaceState.graph` atomically — `workspace.go:521,590,643`.
3. `Workspace.Sync` also publishes fresh graph; old state survives failures.
4. `graph.Clear()` deleted — no matches in `internal/graph`.
5. `TestConcurrentReloadStateSnapshot` iterates graph in readers, passes `-race` locally (`go test -race -run TestConcurrentReloadStateSnapshot ./internal/workspace/`: ok 2.000s).
6. RR-PA4Y and RR-FJH7 both `status=addressed` with resolution text.
7. `go test -race`, `just lint`, `just coverage-check` all green in #351 CI.
