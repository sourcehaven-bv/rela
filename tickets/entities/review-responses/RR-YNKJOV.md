---
id: RR-YNKJOV
type: review-response
title: Action changes rewrite the counts column too
finding: The SQL update path recomputes counts = countsOf(m) on every message change, including action pending, done and clear, not only edits (sqlchannels/sql.go:715-738). The flag survives only because doc round-trips it through docOf and scanMessage (sql.go:175-205). AC 3 tests an edit only. Add an action-change row to testReadsNoUnread so a future doc field drop is caught on the SQL backends. The file backend's action patches never touch counting (filechannels/reads.go:122-123).
severity: minor
resolution: ADDRESSED testReadsNoUnread gains an action change row (pending, done, clear); Approach 2 notes every SQL UPDATE recomputes counts from the round-tripped doc.
status: addressed
---
