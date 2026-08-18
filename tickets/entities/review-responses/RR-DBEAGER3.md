---
id: RR-DBEAGER3
type: review-response
title: Eager derivation made count cards pay for table rows nothing renders
finding: |-
    The Map-based implementation built an entry for EVERY card on any invalidation, regardless of display mode. `breakdowns` bailed early for a card with no `group_by`, so a count card cost O(1) there — but `tableRows` guarded only on `if (!data)`.

    So a `display: count` card over a 5000-row result set executed `[...data.entities]`, a full array copy, plus a `localeCompare` sort when the card carried a `sort:` — to produce a list nothing renders. Measured at ~28ms of mount-and-settle for 20 count cards over 5000 rows, essentially all of it copying arrays that are never read.

    A regression on the exact axis the ticket exists to improve: it removed two wasted passes from breakdown/table cards and added one to every count card.
severity: significant
resolution: |-
    `cardViews` derives per display mode — `card.display === 'breakdown' ? buildBreakdown(...) : []` and the same for `rows` — so a card only pays for the derivation it renders. This falls out of the restructure in RR-DBKEY1 rather than being a separate guard.

    Pinned by 'does not derive table rows for a count card', which asserts zero property reads for a count card carrying a `sort:`. Mutation-verified: removing the display guards fails it.
status: addressed
