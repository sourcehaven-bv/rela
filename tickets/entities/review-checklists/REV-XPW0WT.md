---
id: REV-XPW0WT
type: review-checklist
title: 'Review: Content-scoped edges are served on faces that do not own them'
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

**Review Responses:** <!-- List IDs of review-response entities created, e.g.,
RR-xxxx -->

RR-XEPS33, RR-U5K94W, RR-1A6PUH (significant, addressed); RR-ZE3VM9, RR-XKVSX1
(minor, addressed); RR-T0G52D (minor, deferred to TKT-2528AB); RR-3K7XJF (minor,
wont-fix); RR-1G0AV2, RR-CQIMXG, RR-I2JTI5, RR-D4DY9K (nit, addressed).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
<!-- For each acceptance criterion, state PASS/FAIL with evidence -->

- PASS: the draft-only content edge is not on the published face (views,
entity GET, include, relations routes, relation filter, table columns, export):
`contentedge_face_test.go`.
- PASS: it is not an incoming neighbor of its target in the published world,
for an all-faces and a `policy@published` reader (views, include, list rows,
incoming relation filter).
- PASS: the draft edge is visible on the draft face; identity edges appear on
every face (`TestContentEdges_ViewSectionsServeOnlyTheOwningFace`).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] ~~User-facing documentation updated~~ (N/A: bug fix, no behavior change to document)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

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
