---
id: RR-1WWC0S8
type: review-response
title: history-purge erases job states without purging anything
finding: The CLI erased the job states even when the purge removed no version.
severity: minor
resolution: It now runs only when final.Purged > 0.
status: addressed
---
