---
id: RR-S1ILV8
type: review-response
title: Revocation by config or role change leaks the channel's next activity
finding: Without a store write no full round runs, so a revoked subscriber keeps the topic and gets one frame at the channel's next post.
severity: minor
resolution: A realtime.yaml reload forces a full live round; TestLive_RealtimeReloadRevokes pins the frame at reload time and none afterwards. acl.yaml changes start a new server generation with a new hub.
status: addressed
---
