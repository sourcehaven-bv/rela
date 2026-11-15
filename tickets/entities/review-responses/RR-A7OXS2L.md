---
id: RR-A7OXS2L
type: review-response
title: Rows charged inconsistently on error
finding: Iterator and point reads charge nothing on error, but page, map and slice reads charged len(...) regardless.
severity: minor
resolution: addRowsOK charges no rows for a failed call on every non-iterator read.
status: addressed
---
