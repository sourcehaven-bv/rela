---
id: RR-R8XDZLS
type: review-response
title: A resync hint can take up to 5 s in other processes
finding: MarkAll goes through requestResync and is deferred when another resync ran within 5 s, which can miss the about-a-second target.
severity: minor
resolution: 'Documented in docs/realtime.md: a full recheck another process asks for (for example after restoring a failed delete) can take up to 5 seconds. The limit is kept since it is what stops resync storms.'
status: addressed
---
