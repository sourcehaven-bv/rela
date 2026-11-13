---
id: RR-RT6IR9L
type: review-response
title: Legacy parents have no last_reply_at
finding: Parents with replies from before the migration show no reply time until the next reply.
severity: minor
reason: The file store has no cheap backfill, so all backends agree on zero; the wire type documents that absent can mean unknown and slice 4 renders it so.
status: deferred
---
