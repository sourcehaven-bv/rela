---
id: REV-O98B5D
type: review-checklist
title: 'Review: executeView traversal is ungated: hidden intermediaries leak reachable descendants in _views API, entity-detail sections, and view commands'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

`just test` green across the module. `just lint` 0 issues (fixed: three
misspellings, and a gocognit breach introduced by inlining the load gate —
extracted as `gateLoadedEntities`). `just comment-lint` clean, no unresolvable
doc links. `just arch-lint` OK. `just coverage-check` PASS (total 79.5%).

One `comment-report` duplication finding exists at `viewworld.go` `provenanceFor`
— confirmed pre-existing by re-running the report with this change stashed, so
it is not grown by this diff.

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

The review found two CRITICAL issues in the first cut of the rebased fix, both
verified against the code before acting on them, and both fixed:

- **RR-VW1FAC** — the frontier gate resolved rows with a default-world header
  scan while the loader used the request's world. A faced type stores no row at
  the zero coordinate (BUG-HC6I2T), so faced entities were dropped from the
  frontier even under permit-all. Reproduced first (NopACL returned `[NOTE-1]`,
  dropping the faced `POL-1`), then fixed by threading the world through.
- **RR-VW2FAC** — both gates used the face-blind row gate without pairing it
  with `faceReadable`, so a `policy@published` principal could walk through a
  draft-only node. Masked by VW1FAC until that was fixed, which is why they were
  fixed together.

Self-review note: the diff touches `querybudget_test.go` (a new budget test and
a `recursive` view in the shared budget config) which is adjacent rather than
unrelated — RR-VWBUDG required it, and the existing view budget test never
exercised a recursive rule.

**Review Responses:** RR-VW1FAC (critical, addressed), RR-VW2FAC (critical,
addressed), RR-VWBUDG (significant, addressed), RR-VWLOGS (minor, addressed),
RR-VWNEED (minor, addressed), RR-VWDRY (minor, wont-fix with reason).

**Review Responses:** <!-- List IDs of review-response entities created, e.g.,
RR-xxxx -->

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- _views traversal drops descendants reachable only via a hidden intermediary —
  **PASS**. `TestACLViewTraversal_ViewsAPIDoesNotLeakReachableDescendant`;
  mutation-checked (removing the frontier gate leaks `TKT-VISIBLE` into the
  response body).
- Entity-detail sections behave the same — **PASS**.
  `TestACLViewTraversal_SidePanelDoesNotLeakReachableDescendant`, same mutation
  evidence.
- A `where:` clause cannot infer a hidden entity's property values — **PASS**.
  `TestACLViewTraversal_WhereCannotProbeHiddenProperty`.
- NopACL output unchanged — **PASS**.
  `TestACLViewTraversal_NopACLReturnsFullChain`, plus
  `TestACLViewTraversal_SourceGateMatchesLoaderResolution` which pins that the
  gate and the loader agree about which rows exist (the invariant VW1FAC broke).
- The "chokepoint" comment becomes true — **PASS**. Corrected in
  `views_handler.go`, where the handler now lives.
- A prevention measure is added — **PASS**. `TestViewTraversalIsSourceGated`
  guards both gate sites and both halves of each verdict.

Cost is pinned as well as correctness:
`TestQueryBudget_RecursiveViewTraversalIsSizeIndependent` measures 11 store
reads at both 10 and 50 rows.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix, not an enhancement)
- [x] ~~User-facing documentation updated~~ (N/A: no user-facing behavior change for a correctly-configured principal; the fix removes a leak)
- [x] ~~Docs-checklist marked as done~~ (N/A: no docs-checklist required)

**Docs Checklist:** <!-- e.g., DOCS-xxxx -->

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
