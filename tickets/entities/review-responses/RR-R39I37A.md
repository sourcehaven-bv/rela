---
id: RR-R39I37A
type: review-response
title: Notify fan-out is unbounded on the write path
finding: maxNotifyFanout caps entities per hop but not the recipients; a list property lets one write push into many users' piles synchronously. Dedup was O(n^2) with slices.Contains.
severity: significant
resolution: 'The cascade cuts recipients at maxNotifyRecipients (200) and logs a warning; dedup uses maps. The pushes stay synchronous like add_to_pile: piles are internal state and not an external side effect for jobs.Queue.'
status: addressed
---
