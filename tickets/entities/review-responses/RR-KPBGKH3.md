---
id: RR-KPBGKH3
type: review-response
title: Disconnected client elects itself saver
finding: On disconnect y-websocket removes remote awareness, so every disconnected client elects itself saver and saves its own copy. A merge applied into its document can duplicate text once the CRDT merges on reconnect.
severity: significant
resolution: 'collabRoom tracks online (connected and synced), a client is saver only while online and the non-saver fallback saves only while online. Tests: collabRoom saver election and useBodyCollab offline fallback.'
status: addressed
---
