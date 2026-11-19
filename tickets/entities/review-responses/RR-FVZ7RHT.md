---
id: RR-FVZ7RHT
type: review-response
title: 'set: and defer_scope: source not refused'
finding: 'A set: offer would write the subject around the flow; defer_scope: source makes one snooze hide every task.'
severity: minor
resolution: 'set: offers now fail the load (test case added). defer_scope: source is kept on purpose: it hides only hints, never tasks, and is pinned by TestFlowTasks_SourceScope.'
status: addressed
---
