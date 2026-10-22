---
id: RR-SAT733
type: review-response
title: E2E member assertion is vacuous for the transitions affordance
finding: 'e2e/tests/channels.spec.ts:155-157 and realtime-project.ts:66-72: `contained` is posted by [lead] only, so the member never has it in _actions.post. "No transition types for the member" then holds through post alone and never exercises _actions.transitions narrowing post. The only real coverage of post-but-not-fireable is in Go (dpo-ceiled row). Also expectMessageControl(..., "Edit", false) (channels.spec.ts:172, channels.page.ts:123) is only used negatively, so a broken row locator passes silently. Fix: let the member post `contained` (post: [member, lead]) while the transition keeps roles: [lead], and assert the member sees the type offered but unmarked with no hint. Add a positive Edit check on a plain message from the same user.'
severity: minor
resolution: The e2e spec checks a member sees Contained offered but unmarked and without a hint, and adds a positive Edit check on an ordinary message; 7/7 pass.
status: addressed
---
