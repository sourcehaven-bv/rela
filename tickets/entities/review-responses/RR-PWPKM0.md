---
id: RR-PWPKM0
type: review-response
title: A VALARM on a to-do with no DUE has no anchor — underspecified, clients differ
finding: |-
    RFC 5545 anchors a VTODO's relative VALARM TRIGGER to DTSTART or DUE. This renderer deliberately never emits DTSTART (legal per §3.6.2, and arguably better than Apple's DUE-mirroring), so a relative trigger like -PT9H resolves against DUE — which is the intended behaviour.

    But Todo{Alarms: [...]} with NO Due is constructible today and renders an alarm with no anchor at all. That is an underspecified state clients handle inconsistently.

    Fix: either require Due when Alarms is non-empty (validate or drop the alarm at render time), or document the behaviour explicitly at the Alarms field.

    Noted as sound in the same review, for the record: omitting DTSTART entirely is correct, and since it is never emitted the RFC rule that DUE must be later than DTSTART cannot be violated.
severity: minor
resolution: |-
    Fixed in commit 5a0cac4e. RenderTodo now emits VALARMs only when Due is non-zero. Since this renderer never emits DTSTART, a relative TRIGGER on a to-do with no DUE has nothing to anchor to — dropping the alarm is better than emitting one whose firing time is client-dependent.

    Pinned by TestRenderTodo_AlarmRequiresAnchor. The reasoning is in a comment at the call site citing RFC 5545 §3.8.6.3.
status: addressed
---
