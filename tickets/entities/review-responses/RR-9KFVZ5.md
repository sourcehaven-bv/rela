---
id: RR-9KFVZ5
type: review-response
title: Watch replay cap of 5 misses changes to older messages on a page target
finding: channelTarget caps the replay limit at watchChannelLimit (tools_watch.go:66). A newest-page or before-page target with limit 20 never wakes for an edit, removal or reply count on messages 6-20, although the tool says it returns when the target's result changes. Apply the cap only to targets that follow by after_rev, or say plainly that only after_rev targets are complete.
severity: minor
resolution: The cap now applies only to after_rev and after followers. A page target replays its own limit (still at most 50). Tested by TestWatch_ReadChannelPageTargetSeesOlderEdit and the cases added to TestWatchChannel_TargetAndWakeFilter.
status: addressed
---
