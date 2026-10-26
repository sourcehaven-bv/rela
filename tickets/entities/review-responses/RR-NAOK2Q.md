---
id: RR-NAOK2Q
type: review-response
title: '`{{today}}` in automations is not UTC'
finding: 'automation.Interpolate formats `now` in the clock''s location (internal/automation/template.go:84-85, time.Now()), so the claim that "the vocabulary matches automations" holds for the format only. Plan change: keep UTC for outputs and say that it differs from automations, or use the notice time in UTC and drop the claim.'
severity: nit
resolution: '{{now}} and {{today}} use the notice time in UTC. The plan and docs say this differs from automations, and the matching claim is dropped.'
status: addressed
---
