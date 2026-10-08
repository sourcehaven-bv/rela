---
id: RR-K4DMA9
type: review-response
title: Kanban cross-column drop is two writes
finding: Settle refetch can clobber optimistic order; SSE does not carry relation writes.
severity: minor
resolution: 'Plan updated: Both writes in one mutation sequence; refetch after both; document lack of live update for other clients.'
status: addressed
---
