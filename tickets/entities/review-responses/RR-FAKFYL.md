---
id: RR-FAKFYL
type: review-response
title: Unknown-type early return drops every other literal check and lets reminder post() report ok
finding: internal/realtimecfg/validate.go:960 - one literal naming an unknown-type property returns before any check of the other literals (undeclared names, {{, required). reminders.go:243 then sees no new problem and returns ok, so the reminder is appended to s.reminders. The file is refused anyway, so this is harmless today, but the comment should say the whole check is skipped, not only that property.
severity: nit
resolution: "addressed\tThe comment at the unknown-type early return now says the whole check is skipped for every literal, so the caller sees no new problem."
status: addressed
---
