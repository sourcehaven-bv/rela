---
id: RR-VIAJ1V
type: review-response
title: Damaged rows can hide a truncated statement
finding: 'internal/channels/sqlchannels/waiting.go:42-50: damaged rows count toward the statement''s `LIMIT MaxWaitingRows+1`, but not toward the merged ceiling. If a statement returns exactly LIMIT rows and some are damaged, found can stay at or below the ceiling while rows past the LIMIT were never read. The result is then partial but reads as complete, which is what ErrTooManyRows exists to prevent. Fix: if a statement returns LIMIT rows, return TooManyWaiting whatever the damaged count.'
severity: nit
resolution: A SQL statement that fills its LIMIT returns ErrTooManyRows even with damaged rows; sqlite test added
status: addressed
---
