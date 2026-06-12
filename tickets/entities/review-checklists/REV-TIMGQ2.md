---
id: REV-TIMGQ2
type: review-checklist
title: 'Review: EntityList migration to Pinia Colada (FEAT-XY2D1L slice 2)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (1050 unit tests, 66 files; list+crud+keyboard E2E: 25; kanban E2E: 15, against the built rela-server)
- [x] Lint clean (0 errors, 82-warning baseline — no regression)
- [x] ~~Coverage maintained~~ (N/A: frontend coverage ratchet removed in PR #944)

## Code Review

- [x] Run `/code-review` command (cranky-code-reviewer: 0 critical, 3 significant, 2 minor; verified forward-reference, delete race, SSE liveness, registry boot-gating all correct)
- [x] All critical review-responses addressed (none found)
- [x] All significant review-responses addressed (RR-RIMWZW, RR-135F28, RR-Z7HYW7 — all fixed)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-RIMWZW, RR-135F28, RR-Z7HYW7 (significant, addressed),
RR-Q1CFXV (minor, addressed), RR-ENW3J4 (minor, wont-fix with reason)

## Acceptance Verification

- [x] Each acceptance criterion tested (see PR description)
- [x] Test evidence documented

**Acceptance Status:** PASS — EntityList on useQuery with canonicalized keys;
page-input/meta-output split with server-clamp resync; delete via shared
optimistic helper; Kanban refactored onto it (RR-IVBO9K closed); api/entities.ts
store-free.

## Documentation (enhancements only)

- [x] ~~Docs-checklist~~ (N/A: internal architecture migration; documented on FEAT-XY2D1L + queries/*.ts headers)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass (verified after push)
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/971
