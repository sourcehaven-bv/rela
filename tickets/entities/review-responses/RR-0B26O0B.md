---
id: RR-0B26O0B
type: review-response
title: entitySource.Scan reason misstates the read
finding: Reason said a page at a time; the store read is one unpaged scan batched in the callback.
severity: minor
resolution: Reason reworded to one unpaged header scan handed to the engine in batches of triggerScanPage.
status: addressed
---
