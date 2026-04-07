---
id: RR-MJK3
type: review-response
title: renderListItemTable silently swallows malformed items
finding: If a Lua table item has missing or non-string text field, renderListItemTable produces empty output with no error. Combined with the constructor's pass-through behavior, this means scripts that build task items from dynamic data will silently emit broken markdown when fields are nil or numbers.
severity: critical
resolution: renderListItemTable now coerces non-string text via lua.LVAsString instead of asserting LString. Numbers and other printable values render as their string form. Missing text renders as empty (still). New test TestMdTaskListNonStringText covers the coercion.
status: addressed
---
