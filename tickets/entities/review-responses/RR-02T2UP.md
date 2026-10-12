---
id: RR-02T2UP
type: review-response
title: runSSELoop reaches into liveConn fields
finding: 'Architecture review: the loop read live.notify and took the concrete *liveConn.'
severity: nit
resolution: watcher.go defines liveFrames (wake, takeChanged); the loop and handleSSE use it.
status: addressed
---
