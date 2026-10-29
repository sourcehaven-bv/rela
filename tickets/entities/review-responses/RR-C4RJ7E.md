---
id: RR-C4RJ7E
type: review-response
title: TransitionTo godoc overstates what (state, type, to) identifies
finding: session.go:73-78 says that (state, msgType, to) is "all an outputs mark or the decider's gate reads". resolveMark also reads t.Job() of the resolved entry, and entries without outputs may share a target with a different Job value; see the first finding. Either fix the doc after fixing that finding, or make the invariant true.
severity: minor
resolution: The TransitionTo godoc now says entries without outputs may share a target and differ in Job, and that a reader of Job must pick among Branches; resolveMark does that through markEntry.
status: addressed
---
