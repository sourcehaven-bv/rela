---
id: REV-NM1GT5
type: review-checklist
title: 'Review: Record the archplan baseline and run archplan in CI'
started: "2026-10-28"
completed: "2026-10-28"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] ~~All tests pass (`just test`)~~ (N/A for the full suite: the only Go change is in internal/dataentry/comments_ops.go; `go test ./internal/dataentry ./internal/archguard ./internal/rollup` pass, and CI runs the full suite)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: no new package; one helper extracted from existing tested code)

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

Lint: `just lint` refused to start because another golangci-lint ran in
parallel; `golangci-lint run --allow-parallel-runners` on internal/dataentry and
internal/rollup reports 0 issues. The advisory comment report has no finding in
comments_ops.go.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** cranky-code-reviewer and rela-security-reviewer. Critical:
RR-76PDSQ, RR-ER03OC. Significant: RR-VF5NYV, RR-FB3EZI, RR-4FD6L2, RR-XY1127.
Minor: RR-G4I432, RR-N7NF4V, RR-86Y5AW, RR-E22B9N, RR-02WXTK, RR-NHTZT1,
RR-S14MM3, RR-LMAO3K. Nit: RR-DZ311L, RR-ROLGEM, RR-KGY8N8. All addressed.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. Archplan job passes: PASS locally (CI simulation: 0 new findings, 0
worse metrics); the PR run checks it again.
2. No generic reasons: PASS (412 entries, 0 with "baseline" or no reason).
3. Tuning fixes: PASS (unchecked_entry and the rollup unguarded_path are
gone from `archplan check --all`).
4. Weekly issue dedup and no detail disclosure: PASS by inspection and a
rendered sample body; the first scheduled run confirms it.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: chore)
- [x] ~~User-facing documentation updated~~ (N/A: docs-project has no developer tooling docs; the workflows and justfile carry the instructions)
- [x] ~~Docs-checklist marked as done~~ (N/A: chore)

**Docs Checklist:** N/A

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
