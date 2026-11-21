---
id: IMPL-YUKRW88
type: implementation-checklist
title: 'Implementation: Raise the archplan pins to v26.11.4'
started: "2026-11-21"
completed: "2026-11-21"
status: done
---

## Development

- [x] ~~Unit tests written for new code~~ (N/A: three version pins, no code)
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: three version pins, no code)
- [x] Happy path implemented
- [x] ~~Edge cases from planning handled~~ (N/A: three version pins, no code)
- [x] ~~Error handling in place (errors surfaced, not swallowed)~~ (N/A: three version pins, no code)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: three version pins, no code)
- [x] ~~No hardcoded values in assertions when object is in scope~~ (N/A: three version pins, no code)
- [x] ~~Only specifying values that matter for the test~~ (N/A: three version pins, no code)
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: three version pins, no code)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: three version pins, no code)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] ~~Edge cases manually verified~~ (N/A: three version pins, no code)

**Verification Evidence:**
<!-- Document what you tested and the results -->

## Quality

- [x] Code follows project patterns (check similar code)
- [x] ~~Checked for DRY opportunities — repeated literals, expressions, or~~ (N/A: three version pins, no code)
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] ~~No silent failures (errors logged AND returned)~~ (N/A: three version pins, no code)
- [x] No debug code left behind

**Verification Evidence:**

1. `justfile` names v26.11.4, commit 7923880257f787e7c1c4d3afc31dad62f41d42f9 and exporter v0.2611.4; both tags point at that commit.
2. `archplan --version` at that commit prints `archplan 26.11.4`.
3. Route trace (108 diagrams) and `archplan ci --all` with the new version: 364 accepted, 10 new, 0 ratchet metrics worse, 6 better (`weight.structure` 859 to 180).
The 10 new: 5 `data.orphan_table` that are kept out of the baseline on purpose;
3 `security.untraced` from code newer than the baseline (#2449, #2457); 2 `structure.repeated_orchestration` that replace
14 accepted ones. The weekly job reports only new security findings and worse
metrics, so the pin change adds nothing to its report. The baseline is
unchanged; the two structure findings wait for the owner's decision.
