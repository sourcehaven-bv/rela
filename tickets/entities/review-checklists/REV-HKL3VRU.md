---
id: REV-HKL3VRU
type: review-checklist
title: 'Review: Remove peer keys; machine clients authenticate at the identity edge'
started: "2026-11-08"
completed: "2026-11-09"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`, run inside `just ci`)
- [x] Lint clean (`just lint` for default, sqlite and postgres tags: 0 issues)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`, inside `just ci`)

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
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed (RR-W983COM, RR-VNAJZ2S)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** Design review: RR-W983COM, RR-0Q2WP03, RR-3KMSQNJ,
RR-37W1P5N (addressed). Code review: RR-VNAJZ2S, RR-UUVL6J2, RR-CV9DKK9,
RR-C1KXARB, RR-HB88NRK, RR-ZQPQNQE, RR-Y8HOYM1, RR-PEG55V8, RR-A1IO3F3
(addressed); RR-Q09NYPQ (deferred); RR-00VSZCT, RR-KT1PVVG (wont-fix).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- AC1 PASS: `go build ./...` and `just arch-lint` pass; no `peerkey` in Go code.
- AC2 PASS: `reserved_principal_test.go` lists only `system:` and `integration:`.
- AC3 PASS: `TestReconcileDiff_TokenSecret` sends the stored value as bearer and
refuses `token_key`, `api_key`, missing and empty secrets; a manual run against
a header-mode rela-server answered "No differences".
- AC4 PASS: `TestScriptReadable`, `TestFilterSecrets_DropsUnreadableSecrets`,
the metamodel capabilities row and `TestScriptConfig_RefusesUnreadableSecrets`.
- AC5 PASS: no "Peer keys" section; GUIDE-acl-security states the gate key and
the shared header-mode key; `just docs` regenerated `docs/`.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-2HGHHC2

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: the PR follows done, see TKT-UFV01M)

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
