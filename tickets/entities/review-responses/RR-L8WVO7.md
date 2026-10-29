---
id: RR-L8WVO7
type: review-response
title: Type-label warning also compares trigger-only types
finding: 'internal/realtimecfg/buttons.go:384 compares the label against every type with an on: entry in the state, including types whose entries are all by: trigger. Those never get a type button, so the warning (''whose own button may show beside it'') is a false positive there. Fix: skip types whose entries are all TransitionByTrigger.'
severity: nit
resolution: 'The type-label warning now skips message types whose entries in the state are all by: trigger (helper typeLabeled, which also brought states() under the gocognit limit). Added a test that pins no warning for a trigger-only type and the warning once the entry is person-fired.'
status: addressed
---
