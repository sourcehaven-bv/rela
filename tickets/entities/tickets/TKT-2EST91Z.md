---
id: TKT-2EST91Z
type: ticket
title: Postgres search sorts every match when the needle shares no trigram with any title
kind: enhancement
priority: low
status: backlog
---

## Description

Follow-up to TKT-M58EQ1W (RR-SZ2EJKZ). Postgres search orders hits by
`search_title <-> needle, id`. Every title that shares no trigram with the
needle has distance 1.0. When fewer than `limit` titles are nearer, the
incremental sort reads the whole 1.0 tie group: nearly every match for a word
that is common in bodies.

On the perf seed this does not happen for real words, because they share
trigrams with some titles. It does happen in two cases:

- a needle with no trigram in common with any title;
- the minutes after upgrading to migration 0047, before the first heal pass
has filled `search_title`.

Both cost what every search cost before TKT-M58EQ1W, so this is not a
regression.

## Approach to explore

Split the statement at the tie: rows with distance below 1 through the GiST KNN
scan, then rows at distance 1 in id order, as a `UNION ALL` with the keyset
carried across. The KNN branch needs an index condition that stops it at
distance 1 (for example `search_title % needle` with a near-zero
`pg_trgm.similarity_threshold`), which needs a per-query setting.
