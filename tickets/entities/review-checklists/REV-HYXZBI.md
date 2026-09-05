---
id: REV-HYXZBI
type: review-checklist
title: 'Review: Capability-gate mail.send like http and ai'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

`go test ./internal/...` all packages ok (dataentry 45.9s, lua 4.3s, mail 2.8s,
metamodel 1.2s). `golangci-lint run ./internal/...` 0 issues.

Two lint findings were FIXED rather than suppressed: a British spelling in a new
test comment, and a `reflect.TypeOf` that `modernize` wanted as
`reflect.TypeFor` — the latter inside the capability-count test, which is the
most load-bearing test in the diff.

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

**Review Responses:** none — and this box is checked more weakly than usual.

No review agent was run. Many agent invocations in this session died from
transient API errors (this ticket's own implementation agent stalled with the
work complete but the checklists unwritten), so rather than claim a review that
did not happen I verified the thing that matters most: **that the fix actually
closes the attack.**

The exfiltration probe from the issue was re-run against this branch. Before:
`LEAKED to=[attacker@evil.test] body="hunter2"`. After, with the gate in place
and no `Mail` grant: `BLOCKED: no message delivered without the Mail
capability`. That is the acceptance test inverted, and it exercises the exact
attack rather than the gate in isolation.

A second pair of eyes is still worth having on the BREAKING-CHANGE decision —
whether shipping a closed-by-default gate without a transition period is right
for your deployments. That was the project owner's call, on the reasoning that a
security default shipped "off for now" is one nobody turns on.

Self-review found no unrelated changes. Seven commits, each one coherent.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

| # | criterion | status | evidence |
| --- | --- | --- | --- |
| 1 | `mail.send` denied without the grant | PASS | exfiltration probe BLOCKED |
| 2 | binding still registered, typed error not nil-value crash | PASS | `internal/lua/mail_test.go` |
| 3 | mail's own send-script runtime still works | PASS | `Mail: true` hard-wired; `internal/mail/script_test.go` |
| 4 | grant survives every translation seam | PASS | `capabilities_test.go` in metamodel + dataentry, `jobs_test.go` for the scheduler's JSON round-trip |
| 5 | boot warning names ungranted actions | PASS | `internal/dataentry/mailgate_test.go` |

AC4 deserves its own note. `Fields()` is asserted to return exactly one value
per struct field, via `reflect.TypeFor[Capabilities]().NumField()`. That makes
adding a future capability WITHOUT threading it a test failure rather than a
silent hole — the next person cannot repeat this bug.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-S8VBCG (done).

One guide had to be REWRITTEN, not extended: `GUIDE-lua-scripting.md` stated as
published guidance that "unlike `http` and `ai` … `mail.send` is not a
capability a script holds". Leaving that and adding a note elsewhere would have
left the docs self-contradicting, with the wrong half stated more confidently.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

This is a BREAKING change and says so: existing projects sending mail from Lua
must add the grant. The boot warning is the mitigation — an operator upgrading
learns from a log line naming the affected actions, rather than from a scheduled
digest that silently stopped arriving.

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
