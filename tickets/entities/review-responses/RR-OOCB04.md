---
id: RR-OOCB04
type: review-response
title: The move queue stopped after a failed refetch
finding: queue.then(send) never caught, and send awaited invalidateQueries, which rejects when an SSE invalidate aborts the refetch. Every later move was then skipped until remount.
severity: significant
resolution: New createMoveQueue swallows send and settle failures so the next move runs, and settles once the queue is empty. Tested in moveQueue.test.ts and useListReorder.test.ts.
status: addressed
---
