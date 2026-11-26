---
id: REV-H5LESX3
type: review-checklist
title: 'Review: Flows M1f: predicate starts and change events from every write'
started: "2026-11-25"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): all packages pass except two environmental cases. `cmd/rela-desktop` TestChromeStyle needs the built SPA, which this worktree lacks. `internal/dataentry` hit the 10-minute cap under full-suite load and passes alone in 172s. Flow packages also pass with `-tags sqlite` and `-tags postgres`.
- [x] Lint clean: golangci-lint on the changed packages reports 0 issues; arch-lint, plimsoll and comment-body pass.
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained~~ (N/A locally: new code is covered by new tests in flowengine, flowcfg, flowrun and appbuild; CI runs the floor check)

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

**Review Responses:** RR-KWOOXOR, RR-PJNKI5F, RR-Q04MLNO, RR-VCCHQYE, RR-5RJR082, RR-NKYI7RM, RR-9AX6A2V, RR-NSHSC70, RR-F2Y3EXH, RR-8DT15B3, RR-WN7BXI5, RR-RITO2GA, RR-QDK8ZVT, RR-7G21UXD, RR-TMJTK1N, RR-H33UVMV, RR-WF0IRHC, RR-3GYR0T4, RR-CMQ75CI, RR-6FNKJB7. Deferred tests: TKT-FHH2JNJ.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- Every manager update fires `change`: PASS (TestFlowWriteEvents_PatchMovesTheRun, TestFlowWriteEvents_Lua, all backends).
- Author attribution and role gate: PASS (clerk case leaves the run in collect; ViaChange step By).
- Once-only, in the write's transaction: PASS (joined run store; changeRunRetried; TestChanged_KeptWhileSettling).
- Predicate starts on a false-to-true edge: PASS (TestChanged_StartsOnAPredicate, TestFlowWriteEvents_PredicateStart).
- Manual CLI run on the onboarding sample: PASS (status=hired started a run; documents=true finished it).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated (GUIDE-flows, docs/flows.md)
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-K4C2MFX

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr`~~ (N/A here: /pr runs after the ticket is done; see the note below)

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
