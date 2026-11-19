---
id: RR-VWZXRC8
type: review-response
title: Third copy of the safe-title rule
finding: flowsteps.go added a third whole-title helper beside safeDisplayTitle and safeHeaderTitle in dataentry with different semantics (Redacted list instead of presence).
severity: significant
resolution: metamodel.WholeDisplayTitle holds the rule once (presence based). Both dataentry helpers and FlowControl.titles call it. TestWholeDisplayTitle pins it.
status: addressed
---
