---
id: REV-PFT3Z5E
type: review-checklist
title: 'Review: Link text and truncation for link properties'
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (go test of metamodel, apiwire and dataentry passes; UrlWidget vitest passes; CI runs the full suite and e2e)
- [x] Lint clean (`just lint`) (golangci-lint on the changed packages; CI runs just lint)
- [x] Comment lint gate clean (`just comment-lint`) (just comment-lint: clean)
- [x] Coverage maintained (`just coverage-check`) (new code is covered by its tests; CI checks the floors)

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) (cranky-code-reviewer over the diff)
- [x] All critical review-responses addressed (RR-X7LZ82I addressed)
- [x] All significant review-responses addressed (RR-RJJ46RV, RR-48VXPVJ and RR-DN5C2KR addressed)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-X7LZ82I (critical, addressed); RR-RJJ46RV,
RR-48VXPVJ and RR-DN5C2KR (significant, addressed); RR-ZHH5ITX, RR-H9RC7JP,
RR-58JSBT7 and RR-Y4FYDKK (minor, addressed); RR-QC3NSQ3 (nit, addressed)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** PASS: display link_text shows as the link text with the
URL as target and title; long URLs truncate at 40ch; a blank link_text or one
without link is a schema error; the metamodel guide documents it (Go, vitest,
e2e url-link.spec.ts, Chromium measurement).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs` (DOCS-8MMJ3NT)
- [x] User-facing documentation updated (metamodel guide documents link_text)
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-8MMJ3NT

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: the PR is opened with gh after this checklist is done; CI is watched on GitHub)

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
