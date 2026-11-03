---
id: RR-RP4N6QL
type: review-response
title: A channel tab loses its draft on every tab switch
finding: The tab panel mounted with v-if. Composer text is not stored anywhere, so a reply in progress is lost without warning when the user looks at Details; selection, scroll and loaded pages reset too.
severity: significant
resolution: Deferred to TKT-B75I7LH with the tab design; the planned fix is recorded there.
reason: 'Applies only to placement: tab, which moved to TKT-B75I7LH by the approved scope change. This ticket ships only above_relations, where the channel block is always mounted on the page, so the issue cannot occur here; TKT-B75I7LH carries the finding and its fix.'
status: deferred
---
