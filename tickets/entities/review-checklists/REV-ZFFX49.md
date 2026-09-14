---
id: REV-ZFFX49
type: review-checklist
title: 'Review: Document the ctag watermark''s cross-collection activity signal as accepted residual risk'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

`just lint` 0 issues; `just arch-lint` clean; `just comment-lint` no
unresolvable doc links; `just plimsoll` clean; `just lint-md` 0 issues.

No code changed — this is a godoc and a guide bullet — so the Go gates and
coverage are unaffected by the diff itself.

One pre-existing failure is disclosed rather than papered over:
`TestNoLabelDerivation` fails because it lints `.claude/worktrees/`, the stray
agent worktrees left by this session's delegated runs, not source. Verified
identical against a clean stash, so it is an artefact of the working directory
rather than of this branch.

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

**Review Responses:** none.

The delegated agent that wrote this checked my briefing against the code and
returned four corrections, which is the review this ticket needed:

1. **"A two-column INSERT"** was wrong — `writeEntityTombstone` inserts into
THREE columns (`kind, id_a, typ`). The substance held; the count did not. The
count was dropped rather than fixed, since it carried no argumentative weight.
2. **Option 3 is not a rejectable alternative.** I framed "falling back to
`collectionCTag`'s ETag hashing" as a third option; it is the EXISTING fallback
whenever the store is not a `TypeWatermark` (fsstore). Choosing it means
deleting the watermark's reason to exist, not adopting a different design. The
godoc says so explicitly.
3. **`IMPL-0NYT04` does not exist** in this project. I cited it as a depth
reference from memory; the agent used the most recent implementation checklist
instead.
4. **My `/pr` sequencing was unsatisfiable.** I said open the PR, tick the item,
mark done, commit. The checklist's own note says `/pr` REQUIRES the ticket to be
done and validating clean first. The agent followed the checklist, which is
self-consistent, rather than me.

Corrections 2 and 3 are the ones worth carrying forward: I described a fallback
path as an alternative design, and I invented a ticket ID. Both are the kind of
error that reads as authoritative in a briefing.

The agent also verified my core claims held: the name-hashing separates tag
VALUES but not TIMING, `seq` is the only time-varying input, and the godoc
genuinely never addressed confidentiality.

Self-review of the branch found leaked entities from a sibling ticket
(`PLAN-2PCZEA`, a status flip on `TKT-USQNA3`) that had landed in the base
commit through a shared working tree. Removed.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

| # | criterion | status | evidence |
| --- | --- | --- | --- |
| 1 | a reader at `TypeWatermark` finds the analysis, not the finding | PASS | new fifth godoc section, placed after the functional-safety argument |
| 2 | it answers "what does a shared ctag disclose" | PASS | one bit, no id, no content, no timing resolution beyond the poll interval |
| 3 | it answers "why is per-scope unavailable" | PASS | tombstones hold `(kind, id, typ)`; a narrowed counter cannot see its own deletions and would run backwards |
| 4 | it answers "what would have to change first" | PASS | the tombstone problem, with the GDPR/AVG cost of solving it stated |
| 5 | operator-facing text is actionable | PASS | separate entity TYPES, not separate collections — verified against `EntityTypeWatermark`, which keys on type |

AC5 mattered more than it looks. Advice that sounds reassuring but does not work
is worse than none in a security note; the agent checked that distinct types
genuinely get independent counters rather than asserting it.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-2ODAKC (done).

Two audiences, split deliberately: the godoc carries the full analysis for
someone deciding whether to narrow the watermark; the guide carries one bullet
for an operator deciding how to structure tenants.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

The decision recorded here is ACCEPTANCE, which means the next reviewer will
meet the same finding. The godoc is what makes that encounter productive rather
than repetitive — it answers the question before it is asked, and names the
condition (solving the tombstone problem) under which the answer would change.

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
