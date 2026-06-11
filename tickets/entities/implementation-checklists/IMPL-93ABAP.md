---
id: IMPL-93ABAP
type: implementation-checklist
title: 'Implementation: t.Parallel wave: lua, mcp, write-path packages + -shuffle=on in CI'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (N/A shape — the change IS the tests; 250+ top-level conversions, 109 subtest conversions)
- [x] Integration tests written — covered by full-suite `-race -shuffle=on` runs
- [x] Happy path implemented (wave: lua, mcp, entitymanager, automation, autocascade, acl, affordances, validation)
- [x] Edge cases from planning handled (ai_test.go t.Setenv carve-out; lua_timeout_test.go serial with package comment; slog.SetDefault tests serial after review)
- [x] Error handling in place — n/a

## Test Quality

- [x] Using fixture builders or factories for test data (per-subtest runtime construction where shared LStates were unsafe)
- [x] No hardcoded values in assertions — unchanged assertions
- [x] Only specifying values that matter — n/a
- [x] Interpolated values constructed from objects — n/a
- [x] Property comparisons use original object — n/a

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified
- [x] Edge cases manually verified

**Verification Evidence:**

- Per-package `-race -count=2 -shuffle=on` green for all 8 wave packages; lua+acl additionally `-count=4` after review fixes.
- Real shared-state coupling surfaced and fixed: audit_spoofing_test (parent `defer r.Close()` ran before parallel subtests → closed-LState panic), markdown_test (20 funcs sharing one LState + globals), urls_test (7 funcs, shared LState via t.Cleanup fixture — caused wrong-value failures, not races). All moved to per-subtest runtimes. A systematic scan of remaining parent-state sharing classified the rest safe (read-only mockWorkspace, immutable values, production-concurrent resolvers) — confirmed independently by code review.
- Code review caught one critical: TestCacheLoggingNeverLeaksRawKey raced on the global slog logger under parallel (reproduced; local `just ci` failed on it). Fixed by keeping it serial; same for the latent acl variant.
- tparallel lint: 0 issues across the wave; gofmt clean.
- Timing (single serial run → after): lua 3.6→2.5s, automation 2.4→1.3s, autocascade 1.4→0.9s; validation unchanged (~17s — dominated by deliberately-serial timeout tests). Main wins: race-detector pressure and shuffle-based ordering detection.

## Quality

- [x] Code follows project patterns
- [x] DRY — mechanical insertions scripted; per-subtest fixture pattern consistent
- [x] No security issues introduced
- [x] No silent failures
- [x] No debug code left behind
