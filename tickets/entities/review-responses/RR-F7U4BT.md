---
id: RR-F7U4BT
type: review-response
title: Docs do not say that auto steps ignore the operation's condition
finding: "docs/realtime.md:269 (and GUIDE-realtime.md) says \"`condition` must hold for the entity\" without qualification. lifecycleCheck enforces it for requests, but systemWriter.transition (system.go:72) and the sweep's runAutoStep never evaluate op.Conditions. auto.when now adds a second automatic path that bypasses it.\tAn operator writes close: { condition: \"entity.resolution ~= ''\", auto: { when: \"entity.status == 'done'\" } } and expects an unresolved done ticket to keep its room open. rela closes it.\tSay explicitly in the Lifecycle section that `auto.when` and `auto.after` do not evaluate `condition` (the auto rule is its own condition). If the intent is the opposite, evaluate op.Conditions[entityType] against the row in act before moving."
severity: minor
resolution: 'GUIDE Lifecycle section: condition holds for requests only; auto.when and auto.after ignore roles, permission and condition. docs regenerated.'
status: addressed
---
