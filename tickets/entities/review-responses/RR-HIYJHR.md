---
id: RR-HIYJHR
type: review-response
title: CalendarEventChip listed in the ticket's surface list but not converted (scoped out in plan only)
finding: CalendarEventChip.vue:92 is named in the ticket body's 'Affected surfaces' list but was not converted. The planning checklist DID scope it out with a reason (the chip opens an EntityPreviewModal, not a route, so there is no URL to put in a tab), but the ticket body was never updated to match, leaving the two documents contradicting each other.
severity: significant
resolution: Kept out of scope — the reasoning stands and converting it would change deliberate calendar UX (the chip's target is only known to the parent via an 'open' emit, so giving it a real href is a design change, not a mechanical conversion). Updated the ticket body's surface list to state the exclusion and its reason explicitly, so the body and the plan agree.
status: addressed
---

**Finding (C3, significant).** `CalendarEventChip.vue:92` is named in the
ticket's "Affected surfaces" list but was not converted — ten surfaces were
done, this was the eleventh.

The planning checklist DID scope it out with a reason (the chip opens an
`EntityPreviewModal`, not a route, so there is no URL to put in a tab), but the
ticket body's surface list was never updated to match, leaving the two documents
contradicting each other.

**Resolution:** keep it out of scope — the reasoning stands, and converting it
would change deliberate calendar UX — but state that explicitly in the ticket
body rather than leaving the omission silent. The chip's target is only known to
the parent via an `open` emit, so giving it a real href is a genuine design
change, not a mechanical conversion.
