---
id: PLAN-COXFQD
type: planning-checklist
title: 'Planning: t.Parallel wave + -shuffle=on'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined — see TKT-VRZVXW body (wave packages, exclusions, shuffle)
- [x] Acceptance criteria documented: every top-level test in wave packages parallel unless deliberately excluded with a reason; each package green under `-race -count=2 -shuffle=on`; CI + justfile run with `-shuffle=on`

## Research

- [x] ~~Run `/research`~~ (N/A: mechanical test-hygiene change)
- [x] Checked codebase for similar patterns — scheduler and ai packages are the in-tree exemplars (35 + 27 t.Parallel uses); pgstore schema-per-test isolation documents parallel-subtest intent
- [x] Surveyed parallel-hostility up front: grep shows only `lua/ai_test.go` uses t.Setenv in wave scope; package-level test vars are read-only (interface assertions + a const-style map)

## Approach

- [x] Technical approach chosen: insert `t.Parallel()` after each top-level `func TestXxx(t *testing.T) {`; revert excluded funcs; verify per package; one commit per package
- [x] Alternatives considered: converting table subtests too (deferred — most wall-clock is top-level; subtest conversion doubles the review surface for marginal gain); enabling `paralleltest` linter now (rejected — would flag every unconverted package)
- [x] Dependencies identified: branch stacked on test/top-of-stack-smoke (shares internal/mcp test files with PR #956)

## Security Considerations

- [x] N/A — test-only + CI flag change

## Test Plan

- [x] Per-package `-race -count=2 -shuffle=on`; full `just ci` before PR
- [x] Negative cases: t.Setenv-in-parallel is refused by the Go runtime at test time (loud failure, not silent corruption)

## Risk Assessment

- [x] Effort: s. Main risk: surfacing pre-existing shared-state coupling between tests — partly the point; fixed where found, per-package commits keep bisection clean

## Documentation Planning

- [x] N/A (test-only)

## Design Review

- [x] ~~Run `/design-review`~~ (N/A-with-substitute: approach + wave order + exclusions discussed and approved in working session 2026-06-10)
