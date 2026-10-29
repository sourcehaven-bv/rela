---
id: RR-08MZEJ
type: review-response
title: Documented mixed-row deviation is untested
finding: 'busy.go:148-168 returns ok=true when at least one row''s type attaches, and rows of other types then get []. The notes list this as a deviation, but neither internal/realtime/busy_test.go nor internal/dataentry/busy_rows_test.go has a page with mixed types. TestBusy_NotServed "does not attach" covers only the all-unattached case. Fix: add a TestBusy_Anchor or TestBusy_NotServed case with one ticket row and one page row for `run`. Assert ok=true, the ticket''s entry, the page row absent from the map, and exactly one ListWaiting call over one anchor.'
severity: minor
resolution: 'Added TestBusy_Anchor mixed types: a ticket and a page row in one call; ok is true, the ticket gets its entry, the page gets none, one store call with only the ticket anchor.'
status: addressed
---
