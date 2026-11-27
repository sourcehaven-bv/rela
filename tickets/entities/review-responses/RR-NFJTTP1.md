---
id: RR-NFJTTP1
type: review-response
title: Deprecation warning fires for ids and on every request
finding: 'The warning fires for any non-served value including deleted or unreadable ids, so it points operators at the wrong cause, and it is an unthrottled Warn per request per key. Fix: log only when a title actually matched; consider rate limiting.'
severity: minor
resolution: The deprecation warning is logged only when a title value matched, at most once an hour per list type and relation, and never logs the value.
status: addressed
---
