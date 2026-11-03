---
id: RR-OVZABK
type: review-response
title: AC 3 expectations depend on an unstated fixture
finding: '"Owner in plan-review gets [approve, revise, text]" and "runner in working gets [ask, fail, finished, permission, progress, text]" are true only if every other owner and runner type (go, answer, allow, deny, accept, rework, retry, abandon, claim, resume, plan) also declares in:. Under the PoC file as written plus only approve and progress restricted, the sets would be much larger. Spell out the test fixture (the PoC shape with in: on every transition type) in AC 3 so the implementer does not write a fixture that gives different expectations.'
severity: minor
resolution: 'AC 3 now spells out the fixture: the PoC run type with `in:` on every transition-keyed type, listing exactly its `on:` states, progress limited to `[planning, working]`, and text without `in:`.'
status: addressed
---
