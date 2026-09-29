---
id: RR-ZE3VM9
type: review-response
title: Incoming self-edges were under-shown on bare-id surfaces
finding: Incoming edges on the zero-coordinate surfaces were checked against the empty face, so a content self-edge on a faced row showed in the outgoing column but not the incoming one.
severity: minor
resolution: incomingOwnedAtZero / incomingSourceFace use the row's own face for a self-edge. Pinned by TestIncomingOwnedAtZero.
status: addressed
---
