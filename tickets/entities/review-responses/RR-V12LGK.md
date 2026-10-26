---
id: RR-V12LGK
type: review-response
title: StalePending carries no claim data, so the runner cannot filter or cap without a Get per pending mark
finding: 'PendingAction has only Key, Seq, Name and PendingAt. filechannels meta.Pending stores only name and time (ops.go:354), and the sql backend parses only Name and PendingAt from the doc (sqlchannels/sql.go:1355-1361). Since every claimed mark keeps PendingAt = notice time, "unclaimed or claim older than TTL, oldest first, at most 16" needs a Get for every pending mark. That breaks AC 16 ("at most 16 Get calls"). Plan change: add ClaimedAt (or a next-eligible time) to PendingAction and to pendingRef, or write PendingAt = ClaimedAt + TTL on each claim so the existing index orders eligibility. Mention this in decision 5.'
severity: minor
resolution: An outputs mark's PendingAt means eligible-at (claim time + TTL), rewritten on each claim and progress write. StalePending(now) then returns only eligible marks via the existing index, with no new field.
status: addressed
---
