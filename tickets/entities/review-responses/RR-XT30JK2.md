---
id: RR-XT30JK2
type: review-response
title: Indented hand-written closing fence no longer closes
finding: A hand-written closing fence with leading spaces now leaves the block open.
severity: nit
reason: Not a realistic way to write a fence; it is the shape YAML content takes, which is the bug being fixed. Trailing whitespace and CRLF remain accepted.
status: wont-fix
---
