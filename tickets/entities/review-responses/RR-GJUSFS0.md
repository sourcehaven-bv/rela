---
id: RR-GJUSFS0
type: review-response
title: Unset of a state-machine property not refused
finding: A removal becomes an undeclared transition and fails every write
severity: minor
resolution: unsetSchemaErrors refuses a property whose type declares transitions; load-error test case added
status: addressed
---
