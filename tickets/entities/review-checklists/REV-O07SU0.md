---
id: REV-O07SU0
type: review-checklist
title: 'Review: Data classification overlay: labels, combination rules, subject inference, ACL audit (slice 1)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just coverage-check` runs the full suite with -race: all packages ok)
- [x] Lint clean (`just lint`: 0 issues; arch-lint and plimsoll clean; markdownlint 0 issues)
- [x] Comment lint gate clean (`just comment-lint`: no unresolvable doc links)
- [x] Coverage maintained (`just coverage-check`: PASS; internal/classification 92.8%)

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

**Review Responses:** 40 linked via has-review-response: 1 critical, 16 significant, 17 minor, 6 nit. All addressed except RR-8BV462 (minor, deferred: CLI-only audit, no measured slowness). This round: RR-ZFC8Z0, RR-OX96ZJ, RR-Z58MQO, RR-MT9H4M, RR-MYBWZQ, RR-GWQI1J, RR-D0JIPM, RR-D5Q9LQ, RR-976VI0, RR-CKK2XO, RR-2DYR8I, RR-8BV462, RR-G63CX4, RR-2W337B, RR-BXXX43, RR-WU7BPS (cranky); RR-OITUUS, RR-763BKE, RR-55RR72 (security).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist (IMPL-4GYCPP)

**Acceptance Status:**

- AC1 absent file inert: PASS. Existing validate and acl audit tests pass unchanged; lint/report report the absent file; sync creates it (TestSync_Fresh).
- AC2 lint: PASS. Load and lint table tests cover each defect with path and line; validate includes them.
- AC3 sync: PASS. Order, comment keeping (also comment-only files), renames, stale/conflict reporting, idempotence and atomic write are covered by the sync tests.
- AC4 report: PASS. Worked-example tests cover subjects with provenance, subject links, overrides, person-to-person non-merge, opaque-id flag and stable JSON.
- AC5 ACL findings: PASS with a deviation. Covered by TestClassificationFindings*, TestClassificationViewFor, TestACLAudit_Classification*, TestExposure*. Findings are low and never count toward --fail-on/--exit-code (TestACLAudit_ClassificationDoesNotGate).
- AC6 isolation: PASS. arch-lint allows only cli to import internal/classification.
- AC7 offline: PASS for rela classification *. Deviation: rela acl audit still opens the store as before this ticket; the classification part adds no store access.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-VGS033

## Final Checks

- [x] ~~Commit message explains the why, not just what~~ (N/A: not committed yet; the commit is made on the user's request)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: /pr runs after the ticket is done, on the user's request)

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
