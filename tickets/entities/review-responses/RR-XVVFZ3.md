---
id: RR-XVVFZ3
type: review-response
title: PRIORITY and PERCENT-COMPLETE emit out-of-range values, producing invalid iCalendar
finding: |-
    Independently verified: Priority 42 renders PRIORITY:42, Priority -1 renders PRIORITY:-1, PercentComplete 999 renders PERCENT-COMPLETE:999, PercentComplete -5 renders PERCENT-COMPLETE:-5.

    RFC 5545 constrains PRIORITY to 0-9 and PERCENT-COMPLETE to 0-100. PRIORITY:999 is a parse error in strict clients, and because these properties sit inside a shared VCALENDAR body, one nonsense value can make the WHOLE collection unparseable for every other entry in it — a single bad entity poisoning an entire calendar.

    Nothing validates or clamps today.

    Fix: clamp (or reject) at the RenderTodo chokepoint, same argument as the completion normalisation in RR-1W6PSF. Clamping is preferable to rejecting here: a nonsense priority should degrade that one property, not fail the render.
severity: significant
resolution: |-
    Fixed in commit 5a0cac4e. Todo.normalized() clamps PRIORITY to 0-9 and PERCENT-COMPLETE to 0-100 via named constants (minPriority/maxPriority/minPercentComplete/maxPercentComplete) that cite the RFC sections.

    Clamping rather than rejecting, as recommended: a nonsense priority degrades that one property instead of failing the render for every other entry sharing the VCALENDAR body.

    Note a below-range value clamps to 0, which is the "no information" value for both properties, so the property is omitted entirely rather than emitting a meaningless PRIORITY:0. Pinned by TestRenderTodo_ClampsOutOfRangeValues, including the omit-on-zero subtests.
status: addressed
---
