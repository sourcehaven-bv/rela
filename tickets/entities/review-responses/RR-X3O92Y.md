---
id: RR-X3O92Y
type: review-response
title: Trigger-fired transitions with outputs have no defined writer
finding: 'A trigger post fires transitions with `by: trigger` or with no poster rule (internal/realtime/system.go:50-57, poster{trigger: true}), and its author is systemAuthor() = system:realtime (internal/realtime/sweep.go:90). The plan says a message-driven output writes as "the poster", so a trigger-fired output would write as system:realtime: the identity the plan itself refuses as run_as (decision 2, IsRelaIdentity), with no pre-check, no grant check at load, and a retry principal rebuilt from VerifiedFrom("system:realtime"). With acl.yaml it always fails `forbidden`; without acl.yaml it writes attributed to rela''s own worker. Plan change: treat a transition a trigger can fire (by: trigger, or no roles while a trigger rule posts that message type) as writing as session.run_as, require run_as for it at load, run its grant check (AC 3), and run its mark through the runner like a timer mark. Alternatively refuse outputs on such transitions. Add TestOutputs_TriggerPost and a row in TestParse_SessionOutputsInvalid.'
severity: significant
resolution: '5a refuses at load outputs on a by: trigger transition, or on one without roles whose message type a trigger posts, naming 5b. The run_as writer for such outputs is deferred to 5b.'
status: addressed
---
