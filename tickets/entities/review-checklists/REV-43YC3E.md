---
id: REV-43YC3E
type: review-checklist
title: 'Review: Show a related entity''s property on a kanban card'
started: "2026-10-15"
completed: "2026-10-15"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (go test on dataentryconfig, dataentry, configedit; Vitest on views, components/common, stores, configure, utils, widgets; Playwright kanban specs after the review fixes; full e2e before them with one unrelated flaky crud toast that passes on rerun; `just ci` runs before the PR)
- [x] Lint clean (`just lint` all build tags 0 issues; arch-lint OK; eslint clean apart from the existing KanbanView max-lines warning; vue-tsc clean)
- [x] Comment lint gate clean (`just comment-lint`; markdownlint 0 issues)
- [x] Coverage maintained (new Go code covered by validate_kanban_related_test.go; checked by `just ci`)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed (type-aware redaction test, list values deduplicated per element, logic moved to a composable)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-F3S1IT, RR-WALXQI, RR-RTBZ9F (significant, addressed);
RR-MN3CR5, RR-INNN5B, RR-E4X4LI, RR-M0LYE9, RR-MWWFPV, RR-UQ5R7R (minor,
addressed); RR-HY0XGQ (minor, deferred: a side check on title-only relation
fields can stop existing deployments from booting); RR-DYWKI2, RR-0UQQUG (nit,
addressed); RR-6M72NA (nit, wont-fix).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. Incoming related enum as a styled badge: PASS (KanbanView.relatedProperty.test.ts badge--blue; kanban-related-property.spec.ts "in_progress" orange).
2. Each distinct value once: PASS (Vitest three orgs, two values; list elements; same value through two types; e2e three tasks show two badges).
3. Related currency and date formatted: PASS (Vitest formatAmount and formatDate; e2e "Fix value: € 3.500").
4. Hidden related property absent: PASS (TestListIncludeIncomingPeer_StripsHiddenProperty with a per-type resolver; Vitest missing property and missing peer show nothing).
5. Config load rejects invalid config: PASS (TestValidateConfig_KanbanRelatedProperty; calendar and gantt refusal cases).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-WDKF58

## Final Checks

- [x] Commit message explains the why, not just what (minimal one-line message per the user's instruction; the ticket holds the why)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (done after the ticket is done)
