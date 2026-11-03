---
id: RR-KJSCP4
type: review-response
title: The shadowing predicate is written three times
finding: branches.go:46 (firesFor), :81-83 (shadowed in compile) and :146-148 (unreachable) each spell !e.guarded && entryAdmits(e, who) over a prefix. Extract shadowed(prefix []Transition, who posterClass) bool so the rule the decider follows has one definition on the config side.
severity: nit
resolution: Extracted shadowed(prefix, who), which firesFor, compile and unreachable now use.
status: addressed
---
