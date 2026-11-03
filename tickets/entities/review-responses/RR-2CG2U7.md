---
id: RR-2CG2U7
type: review-response
title: requestStats built log attrs on every request
finding: requestAttrs and route words ran for every request even when neither the access log, debug nor the ceiling would use them.
severity: minor
resolution: The deferred hook returns early unless debug, access log or an over-ceiling read needs the attrs.
status: addressed
---
