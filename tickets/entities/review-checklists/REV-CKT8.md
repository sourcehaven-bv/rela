---
id: REV-CKT8
type: review-checklist
title: 'Review: Response-level action affordances: backend declares per-resource verbs to drive UI'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Coverage maintained (`just coverage-check`)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** Cranky + architect audits happened against the design
(PLAN-GXC7 references RR-W8ZR and RR-YR4B from earlier rounds). Phase 1
implementation is a direct execution of the post-audit design; no new RRs filed.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- AC1 (read-only verdict) — **PASS**, `TestComputeActions_ReadOnly`.
- AC2 (NopACL verdict) — **PASS**, `TestComputeActions_NopACL`.
- AC3 (bidirectional contract) — **PASS**, `TestAffordances_BidirectionalContract` across NopACL + ReadOnlyACL.
- AC8 (no audit noise) — **PASS**, `TestComputeActions_NoAuditNoise` against `audit.NewMemory()`.
- AC9 (scope-of-invariant) — **PASS**, documented in `docs/data-entry/api-reference.md` and `docs/security.md`.
- AC10 (structural same-code-path) — **PASS**, `TestNoStrayWriteRequestConstruction`.
- AC4, AC5, AC6, AC7 — deferred to phase-2 follow-up ticket (no frontend `_actions` consumer in phase 1 yet).

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: docs were updated in-PR; no separate cycle needed)
- [x] User-facing documentation updated
- [x] ~~Docs-checklist marked as done~~ (N/A as above)

**Docs Checklist:** N/A — docs landed in the same PR.

User-facing docs updated:

- `docs/data-entry/api-reference.md` — new "Action affordances (`_actions`)" section with wire shape, verb vocabulary, anonymous fallback, the cardinal rule, additive evolution, scope of invariant.
- `docs/security.md` — added `_actions`-driven affordance hiding to "What the ACL covers in v0," plus a callout that `_actions` is a UI hint not authorization.
- `CLAUDE.md` — new "Action affordances" subsection under "Authorization."

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/779
