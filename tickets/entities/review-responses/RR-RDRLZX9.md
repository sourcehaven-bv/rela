---
id: RR-RDRLZX9
type: review-response
title: The CLI drops the run id when Start fails late
finding: When Start created the run but its first step failed, the CLI printed only the error, so the user could not find the run.
severity: minor
resolution: rela flow start now names the created run in the error and says it will be retried.
status: addressed
---
