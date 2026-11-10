---
id: REV-177BUU5
type: review-checklist
title: 'Review: Flows M3: policy for edits outside a flow'
started: "2026-11-10"
completed: "2026-11-10"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): the affected packages pass with `-race` (appbuild, dataentry, dataentrywire, flowcfg, flowedits, flowengine, flowrun on kv/sqlite/postgres, flowview, archguard, affordances, visibility, mcp, entitymanager); CI runs the full suite on the PR
- [x] Lint clean (`just lint`): golangci-lint clean on the changed packages, run serially; arch-lint and plimsoll clean
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`): new package `internal/flowedits` is covered by unit tests; CI enforces the floors

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

**Review Responses:** cranky-code-reviewer: RR-U4Q55KZ, RR-CODIYGN, RR-KJVGQN4,
RR-TUPD940, RR-I3F8C1A, RR-E3Y5N0R (deferred), RR-Y4LXMH6, RR-BNM4ZMM
(wont-fix), RR-UFO9NJH, RR-VW4MTWN (wont-fix), RR-ICZAM2T (wont-fix),
RR-KRD8PCO. rela-security-reviewer: RR-8GXKPA9, RR-1LEDJ4R, RR-3QK26VQ,
RR-81JM2A9. Design review: RR-2CV78VE, RR-3MLJB30, RR-5Y944MH, RR-A3CHADD,
RR-AJZ229R, RR-EGDO17O, RR-EZFAAVH, RR-FCT21N7, RR-LTCZWY0, RR-MY1HH4U,
RR-NGMV5L4, RR-PBC2XS7, RR-PQNXHO7, RR-Q1M2X2S, RR-ZHVS87Y.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- PASS: `outside_edits:` loads with allow, deny and `{permission:}` at flow and state level, and rejects bad keys, states and properties (flowcfg table tests).
- PASS: a denied property is read-only in `_fields`, its PATCH is refused with `field-affordance:read-only:<p>`, and `_transitions` reports `read-only` (dataentry tests).
- PASS: field-gated manager surfaces are narrowed; ungated ones are not (TestOutsideEdits_GatedWrites, TestOutsideEdits_UngatedWritesUnchanged).
- PASS: the run's own step and outputs are exempt; other runs still narrow, and a refused step answers 403 (TestOutsideEdits_RunOwnWrites, TestOutsideEdits_OtherRunStillNarrows).
- PASS: a run lookup error locks every property (flowedits unit test).
- PASS: one run lookup per section, none per list page (budget tests).
- PASS: the affordance profile override is refused with a ceiling (TestFlowEdits_ProfileRefused).
- PASS: a rename keeps the rules (TestOutsideEdits_RenameKeepsRules).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-7OLSEJI

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
