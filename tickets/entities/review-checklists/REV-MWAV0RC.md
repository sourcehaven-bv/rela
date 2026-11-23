---
id: REV-MWAV0RC
type: review-checklist
title: 'Review: Lower ordered and negated desktop notification conditions'
started: "2026-11-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (changed packages: desktopnotify incl. -tags sqlite, queryplan, predicate, predicatefns, archguard, rela-desktop notify tests; repo-wide run left to CI)
- [x] Lint clean (`just lint`) (golangci-lint on changed packages, default and sqlite tags; arch-lint; plimsoll)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A locally: repo-wide run; CI enforces the floors, and the change adds tests only)

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** code review (cranky + security): RR-2ZOVQDO, RR-7PX0P96, RR-FQHHMAO, RR-MQUXSQ7, RR-N42LWHQ, RR-R632GFI, RR-SU6JUJ8, RR-VQCGRJ6, RR-WB0R42I, RR-XZ3LWKG, RR-YDDKCXX. No critical or significant findings; minors addressed, two nits wont-fix with reasons.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. PASS: TestEvaluate_LoweredTreeIsScaleFree.
2. PASS: TestEvaluate_LoweredMatchesGo_{Memstore,FSStore,SQLite}; postgres covered by storetest.RunGraphDifferential in CI (no local server).
3. PASS: TestEvaluate_DateConditionScanIsNarrowedByLoweredConjuncts.
4. PASS: GUIDE-desktop updated, docs/desktop.md regenerated.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-9XF378O

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
