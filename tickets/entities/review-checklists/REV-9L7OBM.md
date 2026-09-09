---
id: REV-9L7OBM
type: review-checklist
title: 'Review: Reject cleartext http:// for plantuml_server_url except loopback'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — full suite green (exit 0)
- [x] Lint clean (`just lint`) — 0 issues
- [x] Comment lint gate clean (`just comment-lint`) — no unresolvable doc links; `just comment-report` shows this diff introduces no advisory findings
- [x] Coverage maintained (`just coverage-check`) — `internal/dataentryconfig` at 90.8%, above its floor. See note below on a local-only failure.

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) — see findings below
- [x] All critical review-responses addressed — none found
- [x] All significant review-responses addressed — none found
- [x] Self-reviewed the diff for unrelated changes — 5 files, all on this change

**Review Responses:** None created — no critical or significant findings. The
review (started as an agent, finished by hand after repeated transient API
failures) raised three points, all handled in this branch:

1. **Over-refusal of the trailing-dot FQDN** (minor, FIXED). `http://localhost.`
is the fully-qualified spelling of the same machine but was rejected.
`isLoopbackHost` now strips exactly ONE trailing dot. Deliberately `TrimSuffix`,
not `TrimRight`: stripping all of them would let `http://localhost..` through.
Mutation-tested — swapping in `TrimRight` reddens `double_trailing_dot_rejected`
and nothing else.

2. **Redirects are out of scope** (minor, DOCUMENTED). An `https://` URL whose
server 302s to `http://` still ships the source in cleartext, and config
validation cannot see it. Now stated in the guide rather than left implied.

3. **The frontend already re-checks the scheme** (nit, NO CHANGE).
`plantUMLImageURL` in `frontend/src/utils/markdown.ts` re-validates http(s) as
defense in depth, citing `validateApp` by name. Deliberately NOT extending the
loopback rule there: its job is to stop a non-http(s) scheme reaching an `<img
src>` with no CSP backstop, and duplicating the loopback rule would couple two
layers for no added protection — config load already refuses those values.

Edge cases probed against the real predicate and confirmed to fail SAFE (all
over-refusals, nothing remote wrongly permitted): `0.0.0.0`, `[::]`,
`fe80::1%eth0`, the decimal `2130706433` and octal `0177.0.0.1` forms of
127.0.0.1. The IPv4-mapped `::ffff:127.0.0.1` and expanded `0:0:0:0:0:0:0:1` are
correctly accepted — both now have test rows.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

All PASS. Verified end-to-end against a locally-built `cmd/rela` running `rela
validate` over a scratch project — the full 16-case table is in IMPL-C3BMVP.

- AC1 https any host accepted — PASS
- AC2 http loopback accepted (`localhost`, `LOCALHOST`, `127.0.0.1`,
`127.1.2.3`, `[::1]`) — PASS
- AC3 http non-loopback rejected (remote name, public IP, RFC1918) — PASS
- AC4 lookalike hosts rejected — PASS; `localhost@evil.com` reports
`got host "evil.com"`, confirming `Hostname()` unmasked the userinfo
- AC5 pre-existing scheme/host errors unchanged — PASS
- AC6 documented in the data-entry guide — PASS

**Note on `just coverage-check`:** it fails LOCALLY, but not because of this
change. `TestNoLabelDerivation` scans the working tree and picks up files inside
`.claude/worktrees/` — nested git worktrees used by parallel agents — reporting
findings in `DynamicForm.vue` and `openapi/paths.go` that belong to another
branch. Both findings are inside worktrees, zero in real source, and the test
passes on a clean `origin/develop` checkout. CI checks out a clean tree, so it
does not occur there.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-IL5IUH

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
