---
id: REV-6I9M07
type: review-checklist
title: 'Review: Comment and relation counts on kanban cards'
started: "2026-10-09"
completed: "2026-10-09"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

Also run: pgcomments against a local PostgreSQL 18, frontend vitest (all files),
vue-tsc, eslint (0 errors), arch-lint, plimsoll, markdownlint, and the kanban,
comments and kanban-card-counts e2e specs.

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

**Review Responses:** RR-ILK8UM, RR-EPSGK8, RR-I4LNTE, RR-DVMMIL (significant,
addressed); RR-PXB8WV, RR-TEMYIO, RR-FFD2UE, RR-RAM3YS (minor, addressed);
RR-YPMQ40 (nits, addressed). A separate security review (rela-security-reviewer)
found nothing: counts are built only from rows that passed the read gate, the
global grant implies the per-entity one, and SQL binds every key.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- AC1 relation count: PASS (e2e `kanban-card-counts.spec.ts`, unit test).
- AC2 comment count: PASS (e2e, handler tests, per-face test).
- AC3 one batched call per page: PASS (budget test at 10 and 50 rows).
- AC4 ACL: PASS (no grant and local-role-only get no count; thread still
readable; security review clean).
- AC5 config validation: PASS (kanban table test; calendar and gantt refuse).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-HSWGI5

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
