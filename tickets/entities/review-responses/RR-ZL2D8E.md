---
id: RR-ZL2D8E
type: review-response
title: Re-reads on the SSE goroutine with a background context
finding: Per-connection re-reads block frames and pings, run after disconnect with no timeout, and connections at different cursors defeat the dedupe (AC8 untestable).
severity: significant
resolution: One App-owned live worker runs rounds at a shared cursor with 4 parallel replays and a 5 s deadline each, on a context StopWatching cancels; streams only write frames.
status: addressed
---
