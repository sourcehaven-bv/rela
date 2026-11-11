---
id: REV-93VMMSB
type: review-checklist
title: 'Review: Two-way sync with a remote: push and held conflicts (9a)'
started: "2026-11-11"
completed: "2026-11-11"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass: `go test -race` on remotesync, reconclient, scheduler, cli and dataentry; the full run is left to CI (BUG-5CS40M2: the local full race run times out in unrelated sqlite packages)
- [x] Lint clean: golangci-lint on every changed package, 0 issues; `just arch-lint` clean
- [x] Comment lint gate clean (`just comment-lint`); the advisory report shows no finding in new code
- [x] Coverage maintained: remotesync 79% (floor 50), httpremote 91% (floor 85), scheduler 84% (floor 50)

**Comment findings.** `just comment-report` lists the advisory rules
(duplication, nil-contract, param-contract, restatement). They are not a merge
gate, but a finding your diff *introduces* should be fixed or suppressed — don't
grow the backlog.

Every rule is a heuristic over prose, so false positives are expected. To
suppress one, prefer the inline form on the declaration line, which travels with
the code and is reviewed in this diff:

```go
func f(p string) {} //commentlint:ignore param-contract  p is contained by Clone
```

Use `.commentlint.yml` (`ignore:` path globs, `allow-phrases:`) only when the
same prose recurs across many sites. A reason is required either way — an
unexplained suppression is a finding nobody can re-evaluate later.

## Code Review

- [x] Run `/code-review` command (cranky-code-reviewer and rela-security-reviewer)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** design review RR-DI92SQY, RR-ZUVW8WM, RR-IS39JPA
(addressed), RR-XGCZSM5 (deferred). Code review: critical RR-3ZLQAGJ,
RR-2W3H3LH; significant RR-XH1T6CQ, RR-TLTTMLZ, RR-4IVD7TK, RR-1N0OTN2; minor
RR-28F1X4Y, RR-XSBPIVF; nit RR-XJHDAHA; after the develop merge, significant
RR-XLXQ0XR (all addressed); minor RR-68KJOJ2
(deferred).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** AC1 PASS and AC2 PASS (`TestRemotePush_EndToEnd`); AC3
PASS (`TestPush_RetriesOnceAfterPreconditionFailed`,
`TestPush_RefusalHoldsTheRow`, `TestPush_CreateMayHaveReachedTheRemote`); AC4
PASS (`TestPush_ConflictIsHeldAndResolved`, scheduler and CLI conflict tests);
AC5 PASS (`TestParse_Errors`).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-HR53OSW

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI

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
