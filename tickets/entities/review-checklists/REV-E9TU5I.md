---
id: REV-E9TU5I
type: review-checklist
title: 'Review: Support relation fields (incoming/outgoing) on kanban cards'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — verified via `just ci` locally (all Go packages `ok`) and PR #1084 CI `Test` job.
- [x] Lint clean (`just lint`) — `just ci` lint `0 issues`; `God-object lint` (plimsoll) OK.
- [x] Coverage maintained (`just coverage-check`) — `just ci` coverage gate PASS (package + total thresholds). New tests: `KanbanView.test.ts`, `KanbanCardField` unmarshal, `list_incoming_contract_test.go`.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) — go-architect + cranky reviews produced 3 review-responses.
- [x] All critical review-responses addressed — RR-M8IIHV (merge-order guard) and RR-UKS8BW (false-green tests → real contract pin) status=addressed.
- [x] All significant review-responses addressed — RR-XM5ZEB (raw-ID fallback) status=addressed.
- [x] Self-reviewed the diff for unrelated changes — diff scoped to kanban card fields (`config.go`, `validate.go`, `KanbanView.vue`, `config.ts`) + tests. Post-rebase cleanup removed a duplicate `containsID` test helper (ODHV2D added an identical one to the package).

**Review Responses:** RR-M8IIHV, RR-UKS8BW, RR-XM5ZEB

## Acceptance Verification

- [x] Each acceptance criterion tested — incoming + outgoing card fields covered by `KanbanView.test.ts`; the cross-ticket incoming contract is pinned by `TestListEndpoint_IncomingEdge_InverseKey_ODHV2DContract`, which flipped from skip → PASS after rebasing on merged TKT-ODHV2D (#1062), proving the incoming path works end-to-end.
- [x] Test evidence documented — see Acceptance Status.

**Acceptance Status:**
- Incoming card field (`{relation: verantwoordelijk_voor, direction: incoming}`) renders the source title — PASS (`KanbanView.test.ts` incoming-via-inverse + contract test live post-#1062).
- Outgoing relation card field renders its target(s) — PASS (`KanbanView.test.ts`).
- Existing property-only card configs render unchanged — PASS (`KanbanCardField` unmarshal test: legacy `- property: X` decodes; property-only board requests no `include`).

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: additive config field; the `card.fields` relation/direction keys mirror the already-documented `ListColumn` direction, and `docs/metamodel.md` is generated.)
- [x] ~~User-facing documentation updated~~ (N/A: no new user-facing surface beyond the config key, which parallels existing documented relation-column direction.)
- [x] ~~Docs-checklist marked as done~~ (N/A: no docs-checklist needed per above.)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what — commits explain the net-new card-relation feature and the RR-M8IIHV/RR-UKS8BW fixes.
- [x] No TODOs or FIXMEs left unaddressed — grep of changed files clean.
- [x] Ready for another developer to use.

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI — PR #1084 open with squash auto-merge.
- [x] All CI checks pass — all code checks green via `just ci`; the `Rela Tickets` gate clears with this commit (ticket → done, this checklist → done).
- [x] PR URL documented below.

**PR:** https://github.com/sourcehaven-bv/rela/pull/1084
