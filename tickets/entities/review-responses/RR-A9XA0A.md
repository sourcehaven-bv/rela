---
id: RR-A9XA0A
type: review-response
title: plan.marked can be stale when the store re-runs the decider
finding: 'internal/realtime/outputs.go:322 and service.go Post: marked is set inside the Mark callback and never reset. If a backend calls Decide/Mark more than once (retry inside write), a final move without a mark still triggers runInline, which logs a spurious "reading the outputs mark after a post failed" warning. Fix: derive "marked" from the stored notice (Outcome.Notice and the action presence) instead of a callback side effect.'
severity: nit
resolution: plan.marked is assigned on every decider run; TestPersonMark_DeciderRerun.
status: addressed
---
