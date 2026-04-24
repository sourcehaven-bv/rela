---
id: REV-MRZPR
type: review-checklist
title: 'Review: MCP create_entity ignores id_type — allows custom ID on short/sequential types'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — `go test -race ./...` green across all 40+ packages
- [x] Lint clean (`just lint`) — 0 issues
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A locally: `go-test-coverage` binary not installed on this machine; CI enforces the thresholds)

## Code Review

- [x] Run `/code-review` command (invoked cranky-code-reviewer agent)
- [x] All critical review-responses addressed — none were critical
- [x] All significant review-responses addressed (RR-SG9MC, RR-K21KR, RR-LPQT1)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:**

Significant (all addressed):
- RR-SG9MC — guard ordering relative to duplicate check
- RR-K21KR — fragile Location-header parsing in dataentry test
- RR-LPQT1 — error message not actionable

Minor (all addressed):
- RR-EXNL6 — mustCreate now returns entity; tests stop coupling to generation
- RR-TTZ7A — added `countEntities` no-persistence assertion
- RR-R64RL — pinned MCP test on "custom ID" substring

Nits / deferred:
- RR-1ZKVX (wont-fix) — shortMetamodel YAML duplication
- RR-NQIOW (wont-fix) — MCP tool description phrasing
- RR-8U3EQ (wont-fix) — explicit IDType in test fixture
- RR-ZZU08 (wont-fix) — unify error paths
- RR-26KB8 (deferred) — typed error sentinel
- RR-A8UMU (deferred) — importer bypasses guard (out of scope)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- AC1 (short + custom ID → error): PASS — `TestCreateEntity_CustomIDRejectedForShort`, `TestHandleCreateEntity_RejectsCustomIDForShortType`.
- AC2 (sequential + custom ID → error): PASS — `TestCreateEntity_CustomIDRejectedForSequential`.
- AC3 (manual + custom ID → success): PASS — `TestCreateEntity_WithCustomID`.
- AC4 (omitted ID → auto-generate): PASS — existing `TestCreateEntity`, `TestGenerateID*` unchanged and green.
- AC5 (MCP surfaces error): PASS — `TestHandleCreateEntity_RejectsCustomIDForShortType` asserts `result.IsError` and full error propagation.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: no user-facing behavior change)
- [x] ~~User-facing documentation updated~~ (N/A per above)
- [x] ~~Docs-checklist marked as done~~ (N/A per above)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass — all green except the validation rule that blocks merging a ticket still in `review`; resolving by transitioning to `done` in the same PR
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/564
