---
id: RR-U274AN
type: review-response
title: Breadth/BodyWatch views drop MentionSources
finding: Tx views forward SoftDelete but not MentionSources; TypeWatermark hidden on all recorders.
severity: minor
resolution: Breadth and BodyWatch views forward MentionSources; TypeWatermark is documented as not forwarded because memstore lacks it, so CalDAV takes its per-entry fallback as on plain memstore.
status: addressed
---
