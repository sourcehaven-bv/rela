---
id: RR-LI5CBJV
type: review-response
title: Title flashes the id and re-reads on any props change
finding: title was set to the id before the read; the watch getter returned a new tuple each run.
severity: minor
resolution: The id shows only after a failed read; the watch uses two sources.
status: addressed
---
