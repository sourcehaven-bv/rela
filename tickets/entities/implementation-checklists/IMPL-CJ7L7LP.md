---
id: IMPL-CJ7L7LP
type: implementation-checklist
title: 'Implementation: Flow guard may read a field every visible: block shows'
started: "2026-11-18"
completed: "2026-11-18"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** Copied the harbourmaster project to /tmp, added a
customer `visible:` block on account that lists terms_accepted and omits org_id,
vat_valid, reverse_charge, terms_accepted_at and eb_relation_id. `rela flow
validate` from this branch: valid. The same with a develop build: refused with
"reads account.terms_accepted, which a visible: grant in acl.yaml restricts".
TestFieldRestricted (acl), TestParse_GuardReadsFieldEveryVisibleBlockShows
(flowcfg), TestParse_ReadsPropertyEveryVisibleBlockShows (realtimecfg) and
TestFieldRestricted_AgreesWithFieldVerdicts (affordances) pass. acl, flowcfg,
realtimecfg, affordances, appbuild and flowengine suites pass.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
