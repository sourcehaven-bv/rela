---
id: RR-XD213E
type: review-response
title: 'Code: per-key lock map grows forever'
finding: One mutex per key was kept for the process lifetime.
severity: minor
resolution: Reference-counted entries are deleted on release. TestAutomationJobs_LocksAreReleased.
status: addressed
---
