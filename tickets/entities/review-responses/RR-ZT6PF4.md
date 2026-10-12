---
id: RR-ZT6PF4
type: review-response
title: Blocked client holds the handler forever
finding: WriteTimeout is 0 and SSE writes have no deadline, so a peer that stops reading blocks its goroutine indefinitely.
severity: minor
resolution: Every SSE write sets a write deadline via http.NewResponseController (now + 2 x ping); ErrNotSupported ignored, other errors end the stream.
status: addressed
---
