---
id: RR-KSX285K
type: review-response
title: 'run_as: system:scheduler passes the run_as check'
finding: Naming the shared scheduler identity explicitly met the non-empty check and defeated the reason run_as is required.
severity: minor
resolution: validateCaptureTask refuses principal.UserScheduler; test case added; guide updated.
status: addressed
---
