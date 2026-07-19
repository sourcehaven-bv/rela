---
id: RR-DENG8U
type: review-response
title: _transitions leaks hidden machine field's current state
finding: 'attachEntityAffordances computes _transitions from the raw entity without consulting FieldVerdicts.Visible. computeTransitions (affordances.go) and PolicyResolver.TransitionVerdicts (affordances/resolver.go) have no visibility check. A field with visible:false has its value stripped from `properties` + rewritten out of `_title`, but `_transitions[field]` is present and lists the out-edges FROM the current value — since a machine''s out-edge set is determined by the current state, an API client reads the hidden status back out of `_transitions`. This opens a new leak channel past the hidden-field invariant, at the API layer (any consumer, not just the SPA). Fix: filter _transitions by the same Visible verdicts (attachEntityAffordances already computes them for _fields/_attachments).'
severity: critical
resolution: computeTransitions now takes the caller's FieldVerdicts and skips any property where !IsVisible(prop), mirroring how computeAttachments already gates hidden file properties. attachEntityAffordances passes the same verdicts it resolved for _fields. Regression test TestTransitionsWire_HiddenMachineFieldOmitted asserts a visible:false machine field is stripped from properties AND absent from _transitions.
status: addressed
---
