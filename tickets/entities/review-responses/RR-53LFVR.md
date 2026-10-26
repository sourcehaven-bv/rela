---
id: RR-53LFVR
type: review-response
title: A lengthened first breaks the documented 'at most one every after the last post'
finding: 'internal/realtime/reminders.go:54-57 (nextSlot) returns since+first whenever Last is before since+first, even when Last is set. Example: every 30m, first 10m, posts at 10m, 40m and 70m. A reload then sets first to 5h. The next reminder is at since+5h, 3h50m after the last post. docs-project/entities/guides/GUIDE-realtime.md:1302-1304 and the plan (decision 5, and ''Alternatives rejected: storing last keeps at most one every of delay'') promise at most one every. TestNextSlot ''a changed first moves the grid'' (reminders_test.go:215) and TestTimers_RemindReload ''a changed first keeps at most one every'' (timers_reminders_test.go:378) only move first to a point before Last, so neither catches this. Fix: when Last is non-zero, use floor division and allow a negative k, so the slot is the first grid point strictly after Last, in (Last, Last+every]. Use since+first only when Last is zero. Add a TestNextSlot row with first 10m changed to 5h and Last at 70m, expecting 85m or another slot within 30m of Last.'
severity: significant
resolution: With a last post, nextSlot takes the first grid slot strictly after it (floor division, negative steps allowed). TestNextSlot gains a first 10m to 5h row and a row with the last post on a grid point.
status: addressed
---
