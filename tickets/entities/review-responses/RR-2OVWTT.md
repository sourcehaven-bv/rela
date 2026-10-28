---
id: RR-2OVWTT
type: review-response
title: Boolean permission gates drop the sentinel
finding: transitions, version tags, comment permission and relation-order gates collapse a RequestFrom failure to false, so callers answer 403 or a refused transition.
severity: minor
reason: They fail closed and return bool by design; changing them means new error-returning interfaces across appbuild and dataentry. Out of scope for this status-mapping fix.
status: deferred
---
