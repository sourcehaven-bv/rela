---
id: RR-VHR7H8K
type: review-response
title: Group keys differ from server axisKey for non-scalar values
finding: String(raw) grouped arrays and objects as their string form; the server maps them to unset.
severity: significant
resolution: groupKey mirrors axisKey (string, finite number, boolean; else unset), with table tests.
status: addressed
---
