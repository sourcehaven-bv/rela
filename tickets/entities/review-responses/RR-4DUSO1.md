---
id: RR-4DUSO1
type: review-response
title: Gate-error semantics change from silent type-drop to hard error
finding: PolicyReader.permittedIDs (policyreader.go:129-144) logs a warning and CONTINUES on a PermitsReadMany error, dropping that whole type fail-closed — the script sees an empty list. Under the pushdown there is no per-type probe to fail, so a gate/store failure surfaces as a GraphQuery error instead. Combined with the TKT-FVQ4 fix (raise rather than break), that turns a previously-silent degradation into a raised Lua error. This is BETTER — silent fail-closed is how TKT-FVQ4 got filed — but it is an unannounced behavior change on the ACL path and should be a documented decision, not an emergent one.
severity: minor
resolution: 'Accepted as a deliberate improvement rather than reverted: a gate failure now raises instead of silently yielding an empty list. Added AC12 pinning it. Rationale recorded — an empty list on gate failure is indistinguishable from ''you may see nothing'', which is the exact ambiguity TKT-FVQ4 exists to remove, so preserving the old silent type-drop would contradict the other half of this same ticket. To be called out in the guide as a behavior change.'
status: addressed
---

Raised by `/design-review` against PLAN-VZXHRJ, before implementation.

Recommend: accept the change, state it in the plan and the guide, and add a test
pinning that a gate failure raises rather than yielding an empty list. An empty
list on gate failure is indistinguishable from "you may see nothing", which is
the ambiguity the whole TKT-FVQ4 ticket is about.
