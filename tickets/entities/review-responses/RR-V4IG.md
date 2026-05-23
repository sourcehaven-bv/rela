---
id: RR-V4IG
type: review-response
title: Partial section update assumes PATCH did not side-effect into properties/relations/mentions
finding: handleCheckboxToggle only copies entry.content into the entry-content section. If a PATCH on content side-effected (via automation) into properties/relations, those sections would display stale fetchView data. True for checkbox-only today, but future reuse of this pattern needs the assumption documented inline.
severity: minor
resolution: Added an inline ASSUMPTION comment in handleCheckboxToggle documenting that the partial-section-update assumes the PATCH only side-effects entry.content. Calls out the failure mode if a future caller (property edits, automation-triggering changes) reuses the pattern and recommends loadView() as the fallback.
status: addressed
---
