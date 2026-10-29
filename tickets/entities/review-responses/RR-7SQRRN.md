---
id: RR-7SQRRN
type: review-response
title: A removed message in a hidden-only run vanishes, and reveal() on it fails silently
finding: 'channelFold.ts:97 sends a removed message to `pending` whenever a run is open, before the keep check. When the run has no collapsed member, closeRun (line 90) drops every member, including the absorbed removed placeholder (this is the deviation the notes record). reveal(seq) (RlChannel.vue:406) then returns true because the seq is held, adds the seq to `revealed`, and that has no effect because quietDisplay never sees the message. A reply quote to such a message is a dead click with no focus and no feedback. Fix: check opts.keep before absorbing a removed message into a run, and in closeRun emit the absorbed removed members as message rows when run.collapsed===0. Add a channelFold.test.ts case and a reveal case.'
severity: minor
resolution: Fixed. foldMessages does not absorb a kept removed message into a run, and closeRun emits the removed members of a run without collapsed messages as their own rows. New channelFold cases "removed inside a hidden-only run is a row" and "kept removed message ends a run", and panel test "reply quote to a removed message between hidden ones" (shown and focused).
status: addressed
---
