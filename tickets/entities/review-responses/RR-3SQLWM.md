---
id: RR-3SQLWM
type: review-response
title: Unreadable-anchor case mixes the anchor gate and the role gate
finding: 'internal/realtime/busy_test.go:261-267 deletes the anchor view under the `private` type, whose read role is entity.owner. With the view gone, the role check fails too, so the test passes even if busy.go:216-218 stopped dropping channels with an unreadable anchor. Fix: run the unreadable-anchor step with the `run` type (read: attached), where only the anchor gate can drop the channel.'
severity: minor
resolution: The unreadable-anchor step now runs as bob on the run type, whose read role is not tied to the anchor, so only the anchor gate can hide the row; the run claim is posted first.
status: addressed
---
