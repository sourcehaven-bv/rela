---
id: RR-A8T9Y1
type: review-response
title: rowsInWorld dropped read errors silently and loaded bodies
finding: rowsInWorld broke out of the loop on an error without a log, and read full rows for titles.
severity: minor
resolution: rowsInWorld reads headers and logs a failed read at debug level.
status: addressed
---
