---
id: RR-3DAO67
type: review-response
title: The budget test cannot see the file backend's per-anchor reads; document ListWaiting's second consumer
finding: 'countingChannels counts calls, but filechannels.ListWaiting walks one face directory per anchor and reads each meta.yaml (internal/channels/filechannels/ops.go:1220-1224). AC 6 therefore pins one call per page but not the O(rows) I/O that makes up most of the live-replay cost on fs. The SQL path is fine: UNIQUE (anchor_id, anchor_face, ...) serves the IN (VALUES) list (internal/store/pgstore/migrations/0026_channels.sql:39). The plan should say this in the godoc and the Risks, and the budget test should include a face-anchored type with rows on two faces and rows whose channel read is denied, so a per-row fallback read would show. WaitingQuery''s godoc says it selects "the channels an inbox looks at" (internal/channels/waiting.go:24-30). Reword it for a second consumer rather than leaving the busy read to depend on inbox wording.'
severity: minor
resolution: The godoc and Risks state the file backend's per-anchor directory and meta.yaml reads, the budget test adds a face-anchored type on two faces and denied rows, and the WaitingQuery godoc is reworded for both consumers.
status: addressed
---
