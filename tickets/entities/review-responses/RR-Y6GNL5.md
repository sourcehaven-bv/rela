---
id: RR-Y6GNL5
type: review-response
title: auto.when lets entity writers close/archive channels past the lifecycle gate and re-close after a reopen
finding: 'internal/realtime/trigger_rules.go rulesFor / trigger.go act: an auto rule moves channels as system:realtime on the enter edge of a condition over the entity''s own properties. Anyone who may write the fields the rule reads can therefore close or archive the entity''s channels without passing lifecycle.close/archive gate: or condition:, and can undo a moderator''s reopen (D2 "a reopen stands") by moving the entity out of and back into the condition (cooldown 1/min only). The channel audit names only system:realtime and auto:<type>/<op>, so the human cause is reachable only by correlating the entity audit. This is the designed semantics (operator-authored rule, correct system attribution), so no code change is required, but docs/realtime.md should state that auto.when makes entity write access sufficient to close/archive, so operators do not treat the lifecycle gate as the only control.'
severity: minor
resolution: GUIDE now states that write access to the properties an auto.when reads is enough to close or archive the entity's channels and to re-close after a reopen, that gates and conditions do not apply, and that the human cause is in the entity history, not the channel audit.
status: addressed
---
