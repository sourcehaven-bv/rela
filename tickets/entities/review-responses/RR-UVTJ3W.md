---
id: RR-UVTJ3W
type: review-response
title: 'Code: foreground jobs deadlock when two keys trigger each other'
finding: In foreground mode run held a non-reentrant per-key mutex and the ctx carried one running key, so A→B→A on one goroutine locked A twice.
severity: critical
resolution: The ctx now carries the set of running keys and a key in the set is skipped; foreground runs take no lock. TestAutomationJobs_ForegroundJobsTriggeringEachOther (mutation-checked).
status: addressed
---
