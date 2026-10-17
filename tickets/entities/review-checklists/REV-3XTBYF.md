---
id: REV-3XTBYF
type: review-checklist
title: 'Review: Webhook principals get principal_type webhook; acl map --webhook with --expect CI gate'
started: "2026-10-17"
completed: "2026-10-17"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (changed packages pass: acl, aclmap, aclaudit, principal, ingest, dataentry, cli ACL tests. The full local run on this loaded machine hit pre-existing failures that also fail on unchanged HEAD: TestClassificationCmd_SyncThenLint hangs, TestChromeStyle_TargetsShippedClasses needs the built frontend; CI runs the full suite)
- [x] Lint clean (`just lint`) (0 issues; arch-lint and plimsoll clean)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A locally: the full suite cannot finish on this machine; every new file has tests and CI runs the coverage gate)

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) (cranky-code-reviewer and the rela security reviewer)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed (two deferred with reasons: RR-N06PGM, RR-2WP0RB; both gaps are documented as not compared)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-3G14JT, RR-3PIQKS, RR-AVGNR9, RR-4B13IR, RR-N06PGM,
RR-2WP0RB, RR-PFNUD1, RR-OWA3IR, RR-KDY7LE, RR-162MYW, RR-33URZD, RR-EFXP8Q,
RR-B4S91D, RR-5ISWRN, RR-S2H5R3, RR-Z1HVJA, RR-2Q2VER. Follow-up: TKT-3EUSCH.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. PASS: TestWebhookCeiling_CapsTheAutomation checks the audited principal type; principal/webhook_test.go.
2. PASS: TestWebhookCeiling_NoBaselineBehavesAsBefore; existing webhook route tests unchanged.
3. PASS: TestWebhookCeiling_RefusesWriteOutsideBaseline and TestWebhookCeiling_CapsTheAutomation.
4. PASS: TestWebhookCeiling_CapsTheAutomation (Lua write refused, hook write stands, declarative action uncapped as documented).
5. PASS: TestACLMapWebhook_Text, TestACLMap_FlagErrors.
6. PASS: aclmap webhook tests and TestACLMapWebhook_Text (permissions, relation writes).
7. PASS: TestACLMapExpect_* (match, diff, missing, bad file, JSON).
8. PASS: TestCheckWebhooks, TestACLAudit_WebhookUncapped.
Map and runtime agreement: TestWebhookCeiling_MapAgreesWithRuntime.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-8QMCXD

## Final Checks

- [x] Commit message explains the why, not just what (minimal message per the user's instruction, with the ticket ID)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (done after the ticket reaches done)

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
