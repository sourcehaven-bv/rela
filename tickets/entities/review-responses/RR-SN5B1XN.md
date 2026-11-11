---
id: RR-SN5B1XN
type: review-response
title: Question cap answers a generic 413
finding: Hitting MaxAsked returned too_large with no hint of the cause.
severity: minor
resolution: New channels.ErrTooManyAsked maps to 409 too_many_questions with a title naming the cap; listed in the guide and OpenAPI.
status: addressed
---
