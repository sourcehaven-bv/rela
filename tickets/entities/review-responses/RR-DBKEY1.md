---
id: RR-DBKEY1
type: review-response
title: Keying derived card data by cardKey renders one card's breakdown on another card's tile
finding: |-
    The first implementation memoized `getBreakdown` / `getTableRows` into `computed` Maps keyed by `cardKey(card)`. `cardKey` hashes `[title, query, display]` — it does NOT include `group_by`, `sort`, `columns` or `limit`.

    That omission is benign for `cardData`, the fetch cache: two cards with the same query legitimately issue one request and share one `{entities, count}`. It is NOT benign for derived data, because the derivation reads exactly the fields the key omits. Two cards colliding on the key both write to `out.set(key, …)` — last writer wins — and both tiles then read that single entry.

    Reproduced independently of the review, with two breakdown cards sharing title/query/display, one `group_by: status` and one `group_by: priority`, over one entity `{status: todo, priority: high}`:

        OLD  tile0 => "T todo1"   tile1 => "T high1"   (correct)
        NEW  tile0 => "T high1"   tile1 => "T high1"   (tile0 wrong)

    Confirmed as a regression, not a pre-existing fault: reverting `getBreakdown` to its non-memoized form makes the same scenario pass.

    This is the exact one-card's-data-on-another's-tile failure that TKT-53KICM introduced `cardKey` to prevent, reintroduced one layer up in the derivation. It is silent — the user sees a plausible chart of the wrong dimension — and "tickets by status" beside "tickets by priority" over one saved query is an ordinary dashboard config.
severity: critical
resolution: |-
    Restructured rather than patched. Instead of Maps keyed by a string, a single `cardViews` computed maps `cards.value` to one `CardView` per card BY POSITION, and the template `v-for`s over that. No key is shared between cards, so the collision class cannot exist — rather than being avoided by widening the key, which would leave the next person free to narrow it again.

    `cardKey` is still used for `cardData` (where sharing a fetch is correct and desirable) and as the `v-for` `:key`. The godoc on `cardViews` states why the two must not be merged.

    Pinned by 'gives two cards differing only in group_by their own breakdown'. Mutation-verified: routing the breakdown back through a cardKey lookup fails it.
status: addressed
