---
id: RR-J8UB19
type: review-response
title: rela schema output ignores the range and changed JSON shape
finding: Text output shows no range; JSON emitted Max null and Min null for every property.
severity: minor
resolution: Text output shows Range for integers; Min/Max carry json omitempty so unset bounds are left out.
status: addressed
---
