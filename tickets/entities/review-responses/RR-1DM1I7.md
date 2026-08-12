---
id: RR-1DM1I7
type: review-response
title: TodoETag field-sensitivity is untested — an ETag blind to Priority passes the suite
finding: |-
    VERIFIED BY MUTATION: zeroing Priority inside TodoETag before hashing passes the entire suite.

    TestTodoETag_StableAndSensitive (vtodo_test.go:227) only checks sensitivity to completion state. The implementation IS correct today (it hashes the full render, so every field participates), but nothing pins that.

    Why it matters: the ETag drives CalDAV conditional requests. Any field the ETag ignores is a field whose edit a client never learns about — a permanently stale entry that no amount of polling fixes.

    Fix: table-drive it — for each field on Todo, mutate it and assert the ETag moves. Same treatment for ETag/Event, which has the identical gap.
severity: minor
resolution: |-
    Fixed in commit 5a0cac4e. Added TestTodoETag_SensitiveToEveryField: table-driven over all eleven Todo fields (UID, Summary, Description, URL, Due, Timed, Status, Completed, PercentComplete, Priority, Alarms), each mutated independently with an assertion that the ETag moves.

    VERIFIED BY RE-RUNNING THE MUTATION: zeroing Priority inside TodoETag previously passed the entire suite; it now fails with "TodoETag unchanged after mutating Priority — a client would never see this edit".

    DEFERRED: the identical gap on ETag/Event is pre-existing on the VEVENT path and out of scope here; noted for the follow-up that also covers the RRULE escaping in RR-4RWHHZ.
status: addressed
---
