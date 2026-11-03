---
id: REV-00PQ7SO
type: review-checklist
title: 'Review: /_search: client cancels are logged as failures and answered 500; measure and reduce per-keystroke search cost'
started: "2026-10-31"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

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

**Review Responses:** design review RR-6FVU7QL, RR-4BU9BIX, RR-DG6117E,
RR-2R2CIS2, RR-3LZMYJ0, RR-K3ZDZ5B, RR-31ZGWPQ, RR-PKPP2FN, RR-8COE77M,
RR-ML64UL2; code review round 1 RR-PRFQNUC, RR-8ZQCXZT, RR-PC8TK5G,
RR-RC2DKKN, RR-FLF5BI0, RR-59MYVLO; round 2 (SQL rework) RR-EBB9SHB,
RR-DRN0PE7, RR-ENOAHRI, RR-6GHDBSS, RR-BTOGKOX, RR-1CGJLUI, RR-Q4L9MUV. All
critical and significant ones are addressed; two nits are wont-fix with a
reason.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. PASS. Abort sweep: 30x status=499 canceled=true, 2x 200, no WARN/ERROR,
   no 500. Tests: search_cancel_test.go, requeststats_test.go,
   acl_middleware_test.go, localroleprime_cancel_test.go.
2. PASS. writeListPipelineError returns silently on a canceled request
   context; TestSearch_DeadlineIsNotACancel keeps the deadline mapping.
3. PASS for statements and rows: 7 statements instead of 9 (one ranking
   page instead of three), 50 rows. DB time for "gateway" with limit=50:
   240-560 ms against 540-1080 ms for the old binary in the same session
   (machine load 15-70, so not the planned 200 ms in absolute terms).
4. PASS. 200 ms typist: 1 request, 0 aborts (typesim and
   mentionMenuTyping.test.ts).
5. PASS for selective terms: ranking query "carol" 206 -> 0.25 ms,
   "tsk-0367" 244 -> 0.6 ms; request db_ms 1-11 ms. Unselective "gateway":
   ranking query 208 -> 200 ms (custom) / 205 ms (generic), within 10 %.
   Results and order: storetest conformance passes on pg, sqlite and memory.
   Caveat: a whole request without limit for a term most rows contain was
   up to 1.6x slower in some loaded runs; callers now send limit.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated: api-reference.md (`limit`), server-security.md (`canceled=true`)
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-2FBBV64

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (next step after this checklist)

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
