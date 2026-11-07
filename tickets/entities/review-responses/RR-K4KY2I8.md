---
id: RR-K4KY2I8
type: review-response
title: Run header could carry values the reader cannot see
finding: The engine reads everything; a value it copies into a run property is visible to every reader of the subject.
severity: significant
resolution: 'Plan changed: the header holds config-derived values only (flow name, version hash, status, state name, timestamps). The schema fixes the property set; the threat model records the rule for the runtime.'
status: addressed
---
