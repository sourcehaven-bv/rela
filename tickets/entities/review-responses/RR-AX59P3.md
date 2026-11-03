---
id: RR-AX59P3
type: review-response
title: Legacy /api/events stream not covered by expiry tests
finding: internal/dataentry/sse_expiry_test.go:90 hardcodes /api/v1/_events. The plan lists the legacy stream as an edge case and router.go:135 routes it to the same handler, but nothing pins it. A later split of the handlers, which the v1-only live-view branch in handleSSE already invites, could drop the expiry from the legacy path unnoticed. Make serveStream take the path and run TestSSEExpiry_StreamEndsAtExpiry as a two-row table over both routes.
severity: minor
resolution: TestSSEExpiry_StreamEndsAtExpiry runs over /api/v1/_events and /api/events; the legacy row expects no live connection.
status: addressed
---
