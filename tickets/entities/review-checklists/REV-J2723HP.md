---
id: REV-J2723HP
type: review-checklist
title: 'Review: Flows M4a: parallel branches'
started: "2026-11-11"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass: go test of flowcfg, flowengine, flowedits, flowrun (kv, sql, sqlite, postgres), mcp, mcpwire, appbuild, dataentry, cli; full vitest (412 files)
- [x] Lint clean (golangci-lint default, sqlite and postgres tags)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained: new code has unit and integration tests; CI enforces the floors

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
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** addressed RR-HZKG44C, RR-QPUIFRZ (significant), RR-J1XVW8S,
RR-G7R7EGF (security), RR-ITW1PF1, RR-EV68J46, RR-7TOC2CT; deferred RR-1D9R0UF,
RR-JDRIMGP, RR-ZRMWUYT, RR-SP9FPDR, RR-5PQO65A, RR-L1V8I3C; wont-fix
RR-7F0RKXR, RR-R2LABVS, RR-RV6KATO, RR-XFYT7IH, RR-9SF3R33.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- Single-state runs unchanged on every backend: PASS (existing flowengine,
  appbuild, dataentry and mcp suites pass; flowruntest on kv, sql, sqlite,
  postgres).
- Tests cover fork, join and backward compatibility: PASS (forks_test.go,
  branches_test.go, flowsteps_fork_test.go).
- flowrun conformance on kv, sqlite and postgres: PASS.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-GABGO03

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
