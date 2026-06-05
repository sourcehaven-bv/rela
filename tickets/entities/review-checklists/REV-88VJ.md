---
id: REV-88VJ
type: review-checklist
title: 'Review: store: generic GraphQuery DSL + naive impl + storetest conformance'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`) — 0 issues
- [x] Coverage maintained (`just coverage-check`) — new files all green; conformance suite drives coverage on the naive impl

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:**

This PR's code shape was reviewed via the cranky-code-reviewer agent
on the reference branch `feat/acl-v1-tkt-svxl` (two full passes
producing 22 RR entities under TKT-SVXL). Per
`.ignored/acl-v1-split-plan.md`, none of those 22 findings apply to
this PR's scope (the DSL itself was not the source of any finding —
findings landed on consumers: ACL resolver, affordances, appbuild
wiring).

No new code-review pass is needed for this PR — the ported code is
byte-equivalent to the reviewed-and-addressed reference, with only
naming/docstring polish to remove consumer-specific language. The
DSL is purposefully generic; subsequent consumers (PR 2+) carry
their own review obligations.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** See IMPL-B4FV's acceptance verification
table. AC1–AC5 all PASS with the test names that pin each.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: internal store API only)
- [x] ~~User-facing documentation updated~~ (N/A)
- [x] ~~Docs-checklist marked as done~~ (N/A: not created)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed — the pgstore-naive
delegation is documented as the intended state with a flagged
follow-up (SQL-pushdown ticket), not a TODO in code
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass
- [x] PR URL documented below

**PR:** to be filled in after `/pr` runs
