---
id: RR-PCE64Y
type: review-response
title: Small redundancies in trigger.go
finding: internal/realtime/trigger.go:26-33 re-exports TriggerTick/TriggerReconcile from realtimelimits only for appbuild, which can import realtimelimits directly. trigger.go:1020 `sortedSet` is a pure alias of sortedKeys. ruleID (368-372) keeps only 4 bytes of the hash; widen it to 8 since the key length is irrelevant. The comment at 864-867 has a line well over the comment width.
severity: nit
resolution: Removed the sortedSet alias, widened the ruleID hash to 8 bytes and rewrote the long comment. The TriggerTick and TriggerReconcile re-exports stay because arch-lint forbids appbuild from importing realtimelimits; a comment now says so.
status: addressed
---
