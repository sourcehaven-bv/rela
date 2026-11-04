---
id: RR-PIG9KY8
type: review-response
title: Totals should be config only; values read from the entry
finding: Entry already carries gated rollups; refresh via GET entity, coalesced, only when totals exist; blank when hidden.
severity: minor
resolution: 'totals: is config only and reads entry properties. A changed row value in a section with totals calls session.invalidate() (coalesced view reload, not a GET of the entity); hidden or unset totals are left out.'
status: addressed
---
