---
id: RR-XZDSLW
type: review-response
title: A later trigger overwrote an unhandled token's hop count
finding: Each trigger replaced the token, so an unrelated save between two reruns lowered the chain's count and the loop could continue past the limit.
severity: significant
resolution: recordTrigger keeps the higher count while the current token is unhandled, under a process-local mutex. TestAutomationJobs_UnhandledTriggerKeepsHops.
status: addressed
---
