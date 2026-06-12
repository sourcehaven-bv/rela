---
id: REV-TIMGQ2
type: review-checklist
title: 'Review: EntityList migration to Pinia Colada (FEAT-XY2D1L slice 2)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (1050 unit tests, 66 files — incl. 7 optimistic-helper, 6 key/canonical, 3 plural; list+crud+keyboard E2E: 25 specs; kanban E2E: 15 specs, against the built rela-server)
- [x] Lint clean (0 errors, 82-warning baseline — no regression; the 2 warnings in touched files are pre-existing max-lines on EntityList/KanbanView)
- [x] ~~Coverage maintained~~ (N/A: frontend coverage ratchet removed in PR #944)

## Code Review

- [x] Run `/code-review` command (cranky-code-reviewer: 0 critical, 3 significant, 2 minor + leverage notes; it verified the listQueryRef forward-reference, delete race, SSE liveness, and registry boot-gating are all correct)
- [x] All critical review-responses addressed (none found)
- [x] All significant review-responses addressed (RR-RIMWZW key collision, RR-135F28 page resync, RR-Z7HYW7 test fidelity — all fixed)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-RIMWZW, RR-135F28, RR-Z7HYW7 (significant, addressed),
RR-Q1CFXV (minor, addressed), RR-ENW3J4 (minor, wont-fix with reason — E2E owns
the prefix contract)

## Acceptance Verification

- [x] Each acceptance criterion tested (useQuery replaces fetchGeneration/scheduleFetch/loading: list E2E; SSE liveness via ['entities',type] prefix: confirmed by reviewer + create-and-see E2E; optimistic delete + rollback + toast: component test + helper units; B1a plural registry + throw: entitiesPlural.test.ts; canonical key collision/order: entities.test.ts)
- [x] Test evidence documented (this checklist + PR description)

**Acceptance Status:** PASS — EntityList on a single useQuery with canonicalized
param keys; page-input/meta-output split with server-clamp resync; delete via
shared optimistic helper; Kanban refactored onto the same helper (RR-IVBO9K
closed); api/entities.ts no longer imports a store. All unit + targeted E2E
green.

## Documentation (enhancements only)

- [x] ~~Docs-checklist~~ (N/A: internal architecture migration; the arc is documented on FEAT-XY2D1L, and queries/*.ts headers document the key hierarchy + helper contract for the next slice)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use (the optimisticList helper + entityKeys.listParams are the reusable surface for the next migrations)

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass (verified after push)
- [x] PR URL documented below

**PR:** (added after creation)
