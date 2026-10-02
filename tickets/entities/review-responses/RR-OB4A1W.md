---
id: RR-OB4A1W
type: review-response
title: Per-cell cache relies on undocumented immutability
finding: The WeakMap keyed on cell objects assumed rows are never mutated in place without saying so.
severity: minor
resolution: Per-cell cache removed; ViewTableCell uses a computed plus a per-PropertyDef widget cache.
status: addressed
---
