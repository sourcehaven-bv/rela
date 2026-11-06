---
id: RR-TDDUR82
type: review-response
title: '[security] Read-only peers can fan out stateless frames'
finding: '[security] ygo relays Hocuspocus BroadcastStateless (tag 6) from read-only peers, so a read-only peer can send 4 MiB frames to every peer in the room.'
severity: minor
reason: The frames are handled inside the upstream ygo relay, changing them needs an upstream change. The per-connection message rate limit bounds the cost meanwhile.
status: deferred
---
