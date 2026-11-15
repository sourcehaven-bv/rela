---
id: IMPL-S5WVD39
type: implementation-checklist
title: 'Implementation: Search time grows linearly with the data on postgres'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (searchtitle_test.go: trigger, heal, sync queue, stop-at-limit)
- [x] Integration tests written (test full flow, not just units) (pgstore suite against postgres, TestSearchExplain on real plans)
- [x] Happy path implemented
- [x] Edge cases from planning handled (empty title, removed type, rename, type change, foreign search_path, schema rename, requeue race)
- [x] Error handling in place (errors surfaced, not swallowed) (SyncSearchTitles fails Open; heal errors are logged and retried next pass)

## Test Quality

- [x] Using fixture builders or factories for test data (existing pgstore test helpers)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end (scale run at scale 1 and 5)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified (EXPLAIN with actual arguments and generic plans)

**Verification Evidence:**

- Scale run, search p50 manager/editor/reader. Before: 493/239/475 ms at
scale 1, 2768/1304/2683 ms at scale 5. After: 9/22/10 ms at scale 1, 28/42/28 ms
at scale 5. Ratio 1.9-3.1 (was 5.6). Statements 6 to 5.
- Seed time unchanged within noise (64/313 s before, 53-65/259-324 s after).
- `go test -race -tags postgres ./internal/store/pgstore`: ok.
- `TestGoldenParity` default and `-tags sqlite`: ok.
- golangci-lint (pgstore, postgres tag), arch-lint, comment-lint, plimsoll:
clean.
- list-tasks at scale 5 was slow in 3 of 5 runs, also with binaries that were
fast in another run: autoanalyze timing, filed as TKT-841NYIH.

## Quality

- [x] Code follows project patterns (check similar code) (trigger lookup follows migration 0021)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced (rela-security-reviewer: one nit, addressed)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
