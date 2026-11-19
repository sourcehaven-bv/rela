---
id: RR-LAFATJM
type: review-response
title: WaitsOnFailedJob copied the rule of Explain
finding: The stored-state check differed from Explain for forked runs, stranded runs and states without a job.
severity: significant
resolution: Replaced by FlowCache.FailedJobs, which reads the flow version and applies the jobFailed rule place() uses; TestFailedJobs_AgreesWithExplain compares both on plain and forked runs.
status: addressed
---
