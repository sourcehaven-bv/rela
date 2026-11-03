---
id: RR-2D3KRA
type: review-response
title: head.last never updated by follow
finding: useChannel.ts follow() refreshes head.channel via setChannel but leaves head.last at the value of the last head fetch. Any consumer that reads head.last (for example a collapsed preview) shows a stale message. Either update last from the merged list or document that consumers must read messages, not head.last.
severity: nit
resolution: useChannel keeps head.last on the newest held message after every merge, with a test.
status: addressed
---
