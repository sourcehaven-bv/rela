---
id: RR-25REQKG
type: review-response
title: 'Trigger ACL check: flow outside_edits locks are bypassed'
finding: Automation writes skip the field gate, and outside_edits is enforced only as a field-verdict ceiling, so a trigger set could change a property a running flow freezes.
severity: significant
resolution: 'newTriggerJudge compiles the flows: section with flowcfg.Parse; a set or create property that a flow''s outside_edits does not allow in some state, or a state writes:, is refused. Test: "a property a flow locks".'
status: addressed
---
