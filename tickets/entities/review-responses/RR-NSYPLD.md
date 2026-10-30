---
id: RR-NSYPLD
type: review-response
title: AC 4 deviation drops retention from the job fixture
finding: 'busy_rows_test.go job fixture declares delete without retention, and the subtests at live_busy_test.go:380-394 go through close, reopen, delete. The explanation in the notes is sound: a delete under retention needs a closed, non-busy channel and cannot move _busy. The retention tombstone path (retention.go:434 Mark) is still covered only through the Local.Mark subtest. Fix: none required. Record the deviation on the ticket and in the fixture comment, which now says ''closes, reopens and deletes it''.'
severity: nit
resolution: 'Fixed (comment only): the job fixture comment in busy_rows_test.go now gives the reason its delete has no retention. The ticket record is left to the coordinator, because this task may not touch tickets/.'
status: addressed
---
