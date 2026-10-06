---
id: RR-D9S15W
type: review-response
title: Retiring the old App was unspecified
finding: SSE loops end only when the broker closes channels (no close method); StartBackground has no stop; CloseAssembly is unsafe while requests still use Jobs (appbuild.go:2185-2190); the save request runs on the old router.
severity: significant
resolution: 'Plan changed: swap, then broker sends config-changed and closes subscribers, an in-flight counter on the old router drains (after the save response), scheduler gets a stop function and is waited on, then CloseAssembly.'
status: addressed
---
