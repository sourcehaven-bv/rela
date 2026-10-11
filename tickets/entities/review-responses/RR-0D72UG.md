---
id: RR-0D72UG
type: review-response
title: pg Stop doc wrong about in-flight statements
finding: Cancelling interrupts the pgx statement; only the unlock still runs.
severity: minor
resolution: Stop doc now says canceling interrupts the statement and the unlock still runs.
status: addressed
---
