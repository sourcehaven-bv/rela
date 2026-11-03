---
id: RR-3YUWO0
type: review-response
title: Baseline "pending" protection covers after_rev only, not after (seq) followers
finding: c.follows is set only for after_rev (tools_watch.go:675). An agent that follows with `after` loses a post that lands between its read and its baseline watch, which is the same race section 13 fixed for after_rev. Extend pending to `after`, or refuse `after` as a watch target for read_channel.
severity: minor
resolution: follows now covers after as well as after_rev. An after follower with a non-empty page is reported at baseline. Tested in TestWatch_ReadChannelBaselineNotEmpty.
status: addressed
---
