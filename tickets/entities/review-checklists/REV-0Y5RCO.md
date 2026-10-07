---
id: REV-0Y5RCO
type: review-checklist
title: 'Review: Lua history API for entity versions'
started: "2026-10-07"
completed: "2026-10-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): `go test ./...` on default and sqlite tags pass except TestChromeStyle_TargetsShippedClasses, which needs the frontend build (fails on a clean develop tree too; passes in CI)
- [x] Lint clean (`just lint`): golangci-lint on changed packages, default and sqlite tags; arch-lint and plimsoll clean
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`): local run stops at the frontend-only test above; CI's coverage job is the gate

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent): cranky-code-reviewer and rela-security-reviewer
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-73AGM0 (critical, addressed), RR-EAX801, RR-998TQ1,
RR-T9ODSO, RR-1F1UXX (significant, addressed), RR-G4K7K5, RR-RX19TC, RR-X7D3VG,
RR-WTSU7U (minor, addressed), RR-LNJY7V, RR-C3NAZX (minor, deferred to
BUG-NFCPUW and BUG-9OK6SL), RR-I0EEAS (nit, addressed)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. PASS: timeline rows (TestHistory_Timeline, TestSQLiteLuaReadsHistory, manual sqlite run). content_hash deliberately dropped after review (RR-73AGM0).
2. PASS: get_version returns the snapshot with version (TestHistory_GetVersionRedacts, TestSQLiteLuaReadsHistory).
3. PASS: hidden and relation-conditional fields redacted (TestHistory_GetVersionRedacts, TestSQLiteLuaHistoryRedactsUnderPolicy).
4. PASS: hidden and missing ids answer nil (TestHistory_MissesAnswerNil).
5. PASS: fs and memory raise, for missing ids too (TestHistory_UnsupportedRaises, manual fs run).
6. PASS: absent version nil; 0, -1, 1.5 and 2^40 raise (TestHistory_MissesAnswerNil, TestHistory_BadVersionRaises).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-D457WN

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
