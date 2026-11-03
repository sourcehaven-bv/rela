---
id: RR-M94QP4
type: review-response
title: AC 6 PoC example cannot happen in the PoC
finding: 'In the PoC realtime.yaml, progress moves no session state. Only claim and resume move it, into planning and working, where the owner has no decisions, so decisionSeq is undefined (useSessionDecisions.ts:151-155). In the PoC the decision-cause kept rule never fires. Change AC 6: use a fixture schema in which a quiet type moves the session into a state with owner decisions, and do not call it the PoC shape.'
severity: minor
resolution: AC 6 now uses a fixture schema with an extra hidden handoff type that moves the session into a state with owner decisions, and no longer calls it the PoC shape. The integration test description matches.
status: addressed
---
