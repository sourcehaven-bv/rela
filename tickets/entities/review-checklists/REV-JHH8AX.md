---
id: REV-JHH8AX
type: review-checklist
title: 'Review: Predicate language: current_user with is_current_user/has_current_user sugar, pushed into next-action queries'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

Full `./internal/... ./cmd/...` suite green; golangci-lint 0 issues; arch-lint,
comment-lint (commented-code + doclink) and plimsoll clean; coverage floors
satisfied at 79.3% total. One doclink finding introduced by the diff (a
bracketed unexported method) was fixed, not suppressed.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:**

Design review (before implementation was accepted): RR-2ITZ84, RR-PDKNZG,
RR-A856MZ (significant, addressed); RR-LD2B23, RR-IZ9XNH, RR-JMHJJ5, RR-QBW3QO,
RR-P1YQPB (minor, addressed); RR-3WKYQX, RR-KUXDQO (nit, addressed); RR-NDER2B
(nit, deferred with reason).

Code review — cranky-code-reviewer: RR-635ZA0 (significant, addressed);
RR-J1XD8R, RR-8JIICP, RR-COK0GT, RR-S7ESOF (minor, addressed); RR-P7424Q,
RR-Z12KRD, RR-LT2O71, RR-1S9AX5, RR-2RD921 (nit, addressed). Code review —
rela-security-reviewer: no findings at any severity; confirmed all four earlier
security fixes complete; one below-threshold note (compare `Tool` as well as
`ID()` once a boundary stamp exists) recorded on TKT-ZQV9O5.

Self-review: the diff also renames three bare `"unknown"` literals to
`principal.Unknown` (needed by the identity guard) and rebuilds the embedded
frontend for the manual check (build artifact, gitignored). Nothing else
unrelated.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. Three spellings compile/evaluate in both exposed profiles; compile error in
a request-less profile — PASS (`TestCurrentUser_EqualityAndSugar`,
`TestCurrentUser_UndeclaredInStdlibProfile`).
2. Absent/empty identity fails closed; empty identity pushes nothing — PASS
(`TestCurrentUser_BindFailsClosed`, `TestMatchesAs_IdentityBoundOnlyWhenNeeded`,
`TestAffordances_CurrentUserSugar_UnknownPlaceholderNeverMatches`,
`TestCondition_IdentityRequiredSkipsOnlyThatSource`, `TestConditionPrefilters`
empty-identity case).
3. Only top-level AND equalities/memberships pushed — PASS
(`TestConstEqualities`, `TestConditionPrefilters`).
4. Pushdown and index inference agree — PASS
(`TestConditionPrefilters_AgreesWithIndexProperties`).
5. Two principals, different entities, end to end — PASS
(`TestNextAction_CurrentUserConditionEndToEnd`; live-server table in
IMPL-69FYMO).
6. All gates green — PASS.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-IRYGXC

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A here: `/pr` gates on
this ticket being `done`, so it runs after this checklist closes — see the note
below.)

<!--
Deliberately NOT tracked here: the PR URL and whether CI passed.

Both post-date this checklist. `/pr` requires the ticket to be `done` and
validating clean before it opens the PR, and a `done` review-checklist may have
no unchecked items — so an item asking for the PR URL can only be satisfied by a
PR that does not exist yet. Checking it early would mean asserting "CI passed"
before CI ran, which turns the checklist from evidence into a formality.

GitHub records both authoritatively, and the branch and commit messages carry
the ticket ID, so the ticket-to-PR link is recoverable without duplicating it
here. See TKT-UFV01M. -->
