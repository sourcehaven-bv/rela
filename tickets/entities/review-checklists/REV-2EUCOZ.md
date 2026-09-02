---
id: REV-2EUCOZ
type: review-checklist
title: 'Review: affordance-grant unknown-key fail-open'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just check`)
- [x] Lint clean (included in `just check`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

`just check` exit 0.

## Code Review

- [x] ~~Run `/code-review` command (invokes cranky-code-reviewer agent)~~
(N/A: not run — see below. Recorded rather than silently ticked.)
- [x] ~~All critical review-responses addressed~~ (N/A: none raised)
- [x] ~~All significant review-responses addressed~~ (N/A: none raised)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** none

The cranky-code-reviewer agent was not invoked. The change is three
unmarshallers delegating to one helper, in the pattern established by the
two already in this file, and it adds no evaluation path. This is the same
call made on BUG-BRZX6O; flagging it twice so the pattern is visible rather
than assumed.

Self-review found and fixed one defect before commit: the first draft of
`rejectUnknownGrantKeys` reported `[%d]` using a counter over the mapping's
keys, so a typo in the *first* grant's second key printed `[1]` — an index
that reads as "the second grant". Replaced with the YAML source line, which
is unambiguous and verified end-to-end (`line 7` for a typo genuinely on
line 7, in two separate blocks).

Scope check: `RoleDef` was deliberately left lenient. A dropped verb key
there removes grants, which fails closed; folding it in would have mixed a
security fix with an ergonomics one. Recorded in the bug rather than left
implicit.

## Acceptance Verification

- [x] Each acceptance criterion tested
- [x] Test evidence documented below

**Acceptance Status:**

1. *A misspelled `when:` is refused in every affordance block* — **PASS**.
`TestLoadPolicy_AffordanceGrantUnknownKey_Rejected` covers `fields:`,
`visible:`, `options:`, `relations:`, the nested `RelationGrant.Fields`
case, and a non-`when` typo (`crate:`).

2. *Supported keys still load* — **PASS**.
`TestLoadPolicy_AffordanceGrantSupportedKeys` round-trips all three
structs, explicitly asserting `RelationGrant.Create` is a non-nil pointer
to false and `Remove` is nil. The new unmarshallers decode via a `type raw`
alias, so this pins that the pointer tri-state survived.

3. *The guard is load-bearing, not decorative* — **PASS**. Mutation-tested:
short-circuiting `rejectUnknownGrantKeys` to `return nil` fails the table
test; restoring it passes. This is the check that BUG-NRCJ9E's first fix
lacked (its test iterated the registry it was meant to guard), so it was
done deliberately here.

4. *End-to-end against the built binary* — **PASS**. Both a `visible:` and
a `relations:` typo are refused at `appbuild` policy load, so the failure
blocks every entry point rather than only the linter.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] ~~User-facing documentation updated~~ (N/A: no documented behaviour
changed. Unlike BUG-BRZX6O — where the guide's example actively suggested
the broken config — no doc anywhere shows a `when:` key spelled wrongly, and
the valid syntax is unchanged. The error message is the operator-facing
surface here.)
- [x] ~~Docs-checklist marked as done~~ (N/A)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
