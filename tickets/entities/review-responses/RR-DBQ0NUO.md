---
id: RR-DBQ0NUO
type: review-response
title: sameJSON fails silently
finding: sameJSON returns false on a decode error without logging.
severity: minor
reason: False means the property is treated as stale and skipped, which is the safe outcome.
status: wont-fix
---
