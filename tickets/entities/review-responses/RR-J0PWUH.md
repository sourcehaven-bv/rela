---
id: RR-J0PWUH
type: review-response
title: Example adds escalate next to the existing escalated type
finding: 'Plan line 88 adds message type escalate to breach-response, which already has escalated (internal/realtimecfg/testdata/valid.yaml:19, the by: trigger move). The example is the docs example (AC 22), and two near-identical names with different roles and moves will confuse readers. Plan change: name it, for example, escalate-stalled.'
severity: nit
resolution: 'Addressed: the example type is renamed escalate-stalled.'
status: addressed
---
