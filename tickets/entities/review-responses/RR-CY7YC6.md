---
id: RR-CY7YC6
type: review-response
title: openForm needs a change for Other in a channel without a session
finding: 'Plan step 8 (line 439-442) calls `openForm(view, generation, live)` with generation undefined in a channel without a session and says formDecision is not set. openForm (useSessionDecisions.ts:273-295) takes a number, compares it with the head''s session generation, and then always sets formDecision. Spell out the change: skip the generation check and the formDecision record when the channel has no session, and add a test that the Other send carries no expect_generation there.'
severity: minor
resolution: 'Step 8 spells out the openForm change: without a session it skips the generation check and does not set formDecision. AC 10 adds a test that the Other send carries no expect_generation there.'
status: addressed
---
