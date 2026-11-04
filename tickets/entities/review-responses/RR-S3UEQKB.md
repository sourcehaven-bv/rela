---
id: RR-S3UEQKB
type: review-response
title: Entity changes re-resolve too often
finding: refreshForChange resolved on every entity:changed on document/page/form pages and on every change of the viewed type, undebounced, even with no context source configured.
severity: significant
resolution: The server returns context_type only when a context source can apply to the page; the client refreshes for that type alone, debounced 1s.
status: addressed
---
