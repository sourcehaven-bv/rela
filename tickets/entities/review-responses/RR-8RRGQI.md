---
id: RR-8RRGQI
type: review-response
title: A restore from tombstone does not note its due time on the DueWake
finding: Lifecycle reopen, Create, Post and the system writer call DueWake.Note. The Untombstone call sites (appbuild/channelguard.go:207,269 and realtime/retention.go:375) do not, so a restored channel whose resumed due time is near can wait up to TimerCeiling. The guide says a timer fires within a second when the last change came from the leading process. Either note the resumed due times, for example by having ResumeSchedule's caller collect them, or narrow the doc sentence.
severity: nit
resolution: Restores note their resumed due time on the DueWake via realtime.Resumption (sweep, guard undo, partial detach); tests cover each path.
status: addressed
---
