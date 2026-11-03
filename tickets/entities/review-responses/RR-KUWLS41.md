---
id: RR-KUWLS41
type: review-response
title: View changed before editAutoSave() could throw; reorder reload did not drop stale results
finding: Checkbox toggle, body input and title save updated viewData before getting the instance. The section-reorder reload wrote a stale view after a route change.
severity: minor
resolution: The checkbox handler gets the instance first; body and title use the pinned instance. The reorder reload drops its result when the type, id or world changed while it was in flight.
status: addressed
---
