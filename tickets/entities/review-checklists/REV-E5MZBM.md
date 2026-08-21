---
id: REV-E5MZBM
type: review-checklist
title: 'Review: acl audit A7 cannot see data-entry.yaml permission gates, so every UI-gating permission is reported dead'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A locally: the recipe
cannot complete on this machine — `internal/dataentry` needs 706s under `-race
-shuffle=on` against Go's 10m per-package default, so the run dies before
reaching the threshold stage. Verified per-package instead: `internal/acl`
80.1%, `internal/aclaudit` 91.2%, `internal/cli` 38.7% against a floor of 30.
Full gate deferred to CI.)

`just lint` → 0 issues. `just arch-lint` → no warnings, which is the
load-bearing check here: it confirms `internal/aclaudit` still imports only
`internal/acl` and that no `dataentryconfig` dependency crossed the boundary.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-8OZ0HS (significant), RR-R7TFQ2 (significant),
RR-NDGXLQ (minor), RR-3Y574H (nit) — all addressed.

No critical findings. Four against this bug:

- **RR-8OZ0HS** — the typed-nil regression was pinned in `aclaudit` (the
callee), not at the CLI call site where the bug actually occurred. Rewriting the
wiring to the wrong form left every test green. Fixed by extracting
`permissionConsumerFor` returning the *interface* type, plus a call-site test
asserting the untyped-nil property. Mutation-verified.
- **RR-R7TFQ2** — the A7 message still named `requires_permission` as the only
consumer, so the remediation was wrong for UI-gated permissions. This is the
half of the original bug report I had missed: the false positive was fixed but
the harmful advice survived. Reworded to name all three consumer classes and
both remediation routes.
- **RR-NDGXLQ** — the adapter bypassed the injected `config.Loader`, silently
disabling A7 on non-OS filesystem backends, and duplicated an existing
`loadDataEntryConfig` helper in the same package.
- **RR-3Y574H** — empty-string emission and a shadowed local.

Self-review: diff confined to `internal/acl`, `internal/aclaudit`,
`internal/cli` plus tracker entities.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. *A permission referenced solely by a UI gate emits no A7 finding* —
**PASS**. Per-surface subtests (document, card, nav, nested nav, command) plus
end-to-end.
2. *A genuinely dead permission is still reported* — **PASS**. End-to-end run
reports `report:sales` with the new message while `history:read` and a nav-gated
permission are correctly silent.
3. *With no PermissionConsumer injected, A7 does not emit "dead"* — **PASS**.
`TestAudit_A7_NilConsumerSuppressesCheck`, plus end-to-end with an unparseable
config: warning emitted, no finding.
4. *Each of the four gate surfaces covered independently* — **PASS**.
Mutation-verified: deleting the nav recursion fails only the nested-nav subtest;
deleting the dashboard loop fails only the dashboard subtest.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix, not an
enhancement)
- [x] ~~User-facing documentation updated~~ (N/A: no doc describes A7's scope —
see RR-R7TFQ2 resolution)
- [x] ~~Docs-checklist marked as done~~ (N/A: no docs-checklist needed)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass — 23/24 green on PR #1375. The single red is the self-referential `Rela Tickets` gate, which fails only because this checklist item was unticked; it clears with this commit and the transition to done.
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1375
