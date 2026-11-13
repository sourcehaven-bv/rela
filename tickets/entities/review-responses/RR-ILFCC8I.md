---
id: RR-ILFCC8I
type: review-response
title: Exposing open questions on read_session wakes the agent on every answer
finding: read_session watch digests its whole result; adding open to SessionView wakes the watch on each answer, breaking the wakes-once criterion.
severity: significant
resolution: 'Plan: the asked set is served only on the HTTP channel head for the SPA, never on read_session; a test pins that three answers do not wake a read_session watch.'
status: addressed
---
