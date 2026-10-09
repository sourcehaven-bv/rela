---
id: RR-HRSM1I
type: review-response
title: 'Design: jobs can loop across automations'
finding: Each job starts a new cascade at depth 0.
severity: significant
resolution: Hop count on ctx and payload; refuse past 8 (TestAutomationJobs_HopLimit); own-write suppression (TestAutomationJobs_OwnWriteDoesNotReschedule).
status: addressed
---
