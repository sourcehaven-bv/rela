---
id: REV-TMEVUT
type: review-checklist
title: 'Review: Entity commenting stage 1: property and section anchors'
status: in-progress
---

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

Full Go suite green (exit 0), `golangci-lint` 0 issues on the touched packages,
`arch-lint` OK, `plimsoll` OK, `comment-lint` clean across 11800 comments,
coverage 79.0% (both thresholds PASS). Frontend: `vue-tsc` clean, ESLint 0
errors, 2142 tests pass, production build succeeds.

The `doclink` gate caught five broken godoc links in the new code. All five were
**fixed, not suppressed**: two named a `PermCommentRead` constant that does not
exist (the real one is unexported, which Go cannot link at all), one said
`EntityDelete` for `EntityDeleted`, and two bracketed unexported methods.

## Code Review

- [ ] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [ ] All critical review-responses addressed
- [ ] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

The eight review-responses linked to this ticket (RR-17JRWP, RR-1PCQ42,
RR-3VSSPM, RR-60067I, RR-7F6NM9, RR-FCUS1V, RR-JT560T, RR-OOPBUZ) are the
**design**-review findings, all folded into the ticket's scope before
implementation. `/code-review` has not yet run against the implemented diff.

**Review Responses:** RR-17JRWP, RR-1PCQ42, RR-3VSSPM, RR-60067I, RR-7F6NM9,
RR-FCUS1V, RR-JT560T, RR-OOPBUZ (all design-stage)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- AC1 (disabled is absent) — **PASS**, live: routes 404, no `.rela/comments/`
created, `/_schema` omits `commentable`.
- AC2 (create + read back) — **PASS**, live.
- AC3 (author/id unforgeable) — **PASS**, live: injected `id`/`author` ignored.
- AC4 (non-commentable type refused) — **PASS**, live 400.
- AC5 (permissions independently enforced) — **PASS** by unit test
(`TestAuthorizer_VerbsAreIndependent`, `TestAuthorizer_OwnVersusAny`).
- AC6 (ownership-conferred permission honoured) — **PASS** by unit test
(`TestAuthorizer_AsksAboutTheTargetEntity`).
- AC7 (read floor, indistinguishable 404) — **PASS**, live 404 +
`TestAuthorizer_ReadFloor`.
- AC8 (mutating perm without read fails at load) — **PASS** by
`internal/acl/commentperms_test.go`.
- AC9 (detached renders, flagged) — **PASS**, unit + wire `detached` flag.
- AC10 (rename re-keys, delete removes) — **PASS**, live: cross-process
`rela rename id` moved the thread to the new id; old id 404s.
- AC11 (concurrent adds both survive) — **PASS** via `commentstest.RunAll`.
- AC12 (body cap / control chars) — **PASS**, live 400 for NUL and empty.
- AC13 (counts post-gate) — **PASS**: the list is authorized before any count,
and the panel's count derives from the returned rows.
- AC14 (both backends pass the suite) — **PASS**: `filecomments` and
`memcomments` both run `commentstest.RunAll`.

## Documentation (enhancements only)

- [ ] Docs-checklist created and linked via `has-docs`
- [ ] User-facing documentation updated
- [ ] Docs-checklist marked as done

This is an enhancement with a new operator-facing config block (`comments:`),
six new ACL permissions and a new HTTP surface, so it needs user-facing docs
before `done`.

**Docs Checklist:** <!-- not yet created -->

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [ ] Run `/pr` command to create PR and monitor CI
