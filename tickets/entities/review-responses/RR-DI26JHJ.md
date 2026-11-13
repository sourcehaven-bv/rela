---
id: RR-DI26JHJ
type: review-response
title: Count and page are two reads
finding: A concurrent write between CountMatched and the page can make Total disagree with Rows.
severity: minor
resolution: The old handler had the same window between reads. Total and has_more are advisory paging hints; a snapshot read per call is a store-wide change out of scope here.
reason: The old handler had the same window between reads. Total and has_more are advisory paging hints; a snapshot read per call is a store-wide change out of scope here.
status: wont-fix
---
