---
id: RR-8SHEV02
type: review-response
title: Record timestamp and duplicate rows unspecified
finding: TS source and retry-after-append duplicates are not defined.
severity: minor
resolution: TS is the run's read time; a retry after a successful append can add a second row and the guide says so.
status: addressed
---
