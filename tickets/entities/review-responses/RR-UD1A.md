---
id: RR-UD1A
type: review-response
title: Date format claim is wrong and would change behaviour in cards/list
finding: |
  Ticket pins Intl.DateTimeFormat with dateStyle:'medium' and calls this "no behaviour change." Two problems. (1) frontend/src/utils/format.ts already exports a formatDate using { year:'numeric', month:'short', day:'numeric' } -- a different format from dateStyle:'medium' (the latter varies by locale: 'Jan 15, 2024' en-US but '15 Jan 2024' en-GB; the existing one is consistent across locales). (2) In EntityDetail.vue cards/list today (lines 642-676), values come out raw via field.values joined or badged -- there is no date formatting at all for non-enum date fields. Both directions are a visible diff.
severity: critical
resolution: |
  Plan revised. Drop the proposed widgets/formatDate.ts -- DateWidget display mode reuses the existing formatDate helper from frontend/src/utils/format.ts. The cards/list date-format-change (raw ISO -> formatted) is documented as deliberate behaviour delta #1 and #2 in the ticket's "Known behaviour deltas" table. Justified as consistency with the existing `properties` mode which already formats dates the same way.
status: addressed
---
