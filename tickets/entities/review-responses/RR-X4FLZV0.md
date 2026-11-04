---
id: RR-X4FLZV0
type: review-response
title: Precedence and route races between tab, channel and seq unspecified
finding: ?tab=chat&channel=run/x is undefined; ChannelsSection onRevealed rebuilds the query (ChannelsSection.vue:202-207); face switch and owner redirect copy the whole query (EntityDetail.vue:1371/1401/1470), so a stale tab survives an entity switch.
severity: minor
resolution: Deferred to TKT-B75I7LH with the tab design; the planned fix is recorded there.
reason: 'Applies only to placement: tab, which moved to TKT-B75I7LH (approved scope change: this ticket ships above_relations only).'
status: deferred
---
