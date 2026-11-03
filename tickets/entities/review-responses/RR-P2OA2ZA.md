---
id: RR-P2OA2ZA
type: review-response
title: Decision glue duplicated and incomplete
finding: Missing DECISIONS_BLOCKED_KEY rule, shared pending keys across rows, a second copy of the refusal mapping, no schema mapping, no lock release on unmount.
severity: minor
resolution: Per-row usePendingKeys; shownPending and refusal mapping extracted to decisionGlue.ts used by OpenChannel and the row; schema maps to channelSchema.reload(); resetDecisions in onScopeDispose.
status: addressed
---
