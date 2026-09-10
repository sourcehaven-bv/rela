---
id: REV-5EYI9P
type: review-checklist
title: 'Review: Writes resolve their face differently from reads, so a create always lands on the bare row'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

Run in two halves rather than one `just test`, because
`TestAnalyzeProperties_StopsScanningAtCap` seeds 5000 entities into an unmerged
in-memory bleve index (documented on `bleveindex.NewMem`) and takes ~15 minutes
on this machine. It is pre-existing, arrived in #1337, and reproduces
identically on clean develop with this branch stashed.

- all packages except `dataentry`: EXIT=0, 103 packages, no failures
- `dataentry` minus that one test: EXIT=0, 74s, 1272 of 1273 tests
- `just lint`: 0 issues; `just arch-lint`: no warnings
- `just comment-lint`: clean across 13923 comments
- `just docs-check`: passes once the regenerated docs are committed

The one test not run locally is unmodified by this branch and runs in CI.

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

**Review Responses:** RR-H0PXTE, RR-WSWB8P, RR-8J9Y30, RR-2D1LJ7, RR-NKQGB2,
RR-8QVKY8

Six criticals. Five were real and fixed; one (RR-2D1LJ7) is accurate but
deferred to TKT-2RQMV4 with the reasoning recorded on the response.

RR-8QVKY8 was found by `just check`, not by the review: the fix for RR-H0PXTE
reintroduced the exact fail-closed ACL gap that
`TestRename_FailsClosedOnNonNotFoundFetchError` exists to pin. `anyFaceOf`
swallowed a transient store error, which the not-found branch then treats as
"absent" and returns before authorizing. Auditing the shape found four more
pre-ACL sites doing the same, three of which pre-date this bug.

RR-NKQGB2 is worth reading before trusting a green cardinality suite: the test
could not fail, and the first attempt to strengthen it produced what looked like
a per-face counting bug. It was a fixture error — an edge tailed on a face with
no entity row is counted by nobody. Both the store and `countRelationsFor` were
correct. The test is now mutation-verified in both directions.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. Writes resolve a face the same way reads do — **PASS**.
`TestCreate_AuthorizesTheFaceItWrites`,
`TestUpdate_ReadsThePreImageAtTheAuthorizedFace`,
`TestApply_ProbesTheFaceTheBodyNames`.
2. ACL applies to the resolved face; a denied face is refused, not
redirected — **PASS**. `TestCreate_DeniedFaceIsRefused`, using an ACL double
that records which faces it was asked about.
3. `bare_face` removed as a modelling key — **PASS**. Gone from metamodel,
aclaudit, shape projection and the SPA; `StoredFace`/`DeclaredFace` deleted
rather than left as identity functions.
4. An unrecognised `face` key is rejected — **PASS**.
`dec.DisallowUnknownFields()` plus `ParseFace` validation;
`TestCreate_UndeclaredFaceIsRefused`, `TestCreate_FacelessTypeRefusesAFace`.

Beyond the report: seven further instances of the same authorize-here/read-there
defect were found and fixed, one of which (`RenameEntity`) was a live ACL
bypass. A separate ID-collision defect (two unrelated entities minting the same
id, which is the ACL row-gate key) was reproduced and fixed across all four
backends.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] User-facing documentation updated
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

Docs were not optional here despite this being a bug: removing `bare_face`
changes a documented modelling key, so the guides would otherwise describe a key
the loader no longer accepts. Updated `GUIDE-content-states`, `GUIDE-metamodel`,
`GUIDE-acl-overview`, `GUIDE-acl-security`, `GUIDE-data-entry`,
`GUIDE-data-migration` and `CON-content-states`, with `docs/*.md` regenerated
from them.

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
