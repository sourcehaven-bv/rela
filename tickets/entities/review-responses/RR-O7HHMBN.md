---
id: RR-O7HHMBN
type: review-response
title: Reconnecting tab refused until its old connection is gone
finding: A client reconnecting with the same id while its old connection was still live was refused until the ping timeout plus a heartbeat.
severity: minor
resolution: A connection with the same owner may take over a live client and then owns its removal on disconnect; covered by TestAwarenessOwnership_BindingOutlivesTheConnection.
status: addressed
---
