---
id: RR-3FUM5QZ
type: review-response
title: rela flow validate messages and nil-safety
finding: validate reports flows.yaml only. The new Transitions accessors need the nil-safety Empty already has.
severity: minor
resolution: validate reports the headless flows on their own line. Every new accessor is nil-safe and has a nil test.
status: addressed
---
