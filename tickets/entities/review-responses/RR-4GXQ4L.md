---
id: RR-4GXQ4L
type: review-response
title: Docs advise one primary per state but decision_button style primary gives one to every preset
finding: 'docs-project/entities/guides/GUIDE-realtime.md:1000 says to give primary to at most one decision per state. Per-key inheritance (buttonoptions.go:65-73, merge at :170-172) copies a type''s style: primary onto every preset of that type, so a type with Approve, Revise and Defer presets in one state shows three primary buttons, plus the type button. Neither the doc nor the load says so. Fix: add one sentence under Button options: style: primary on decision_button reaches every preset of the type that sets no style, so use it on a preset rather than the type when the type has several presets. A load warning is optional; per the plan, a doc note is enough.'
severity: minor
resolution: 'GUIDE-realtime Button options: style: primary on decision_button reaches every preset of the type that sets no style, so with several presets put primary on one preset rather than on the type.'
status: addressed
---
