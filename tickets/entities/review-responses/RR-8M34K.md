---
id: RR-8M34K
type: review-response
title: ORDER BY id uses DB collation, not byte order — diverges from fs/mem backends + keyset risk
finding: 'cranky-code-reviewer #1: pgstore ListEntities/ListRelations/pagination order by id under the DATABASE collation (postgres:16 + Postgres.app default = en_US.UTF-8), but fsstore/memstore order by Go byte-wise comparison (storeutil sorted slices). VERIFIED against live DB: (''A-10'',''a-2'',''A-2'',''B-1'') sorts as ''A-10 a-2 A-2 B-1'' under en_US.UTF-8 vs ''A-10 A-2 B-1 a-2'' under COLLATE C (= Go byte order). Two consequences: (1) the same project served by fs vs pg returns different list order — breaks the store contract''s ''stable, ascending-by-ID'' promise across backends and any export/diff/golden consumer; (2) under a NONdeterministic ICU collation, the keyset cursor (id > $n) is not a total order vs ORDER BY id, dropping/duplicating rows at page boundaries. Conformance missed it: pagination IDs are uniform T-%03d (sort identically under both).'
severity: critical
resolution: 'Fixed: 0001_init.sql now declares all key columns COLLATE "C" (entities.id; relations.from_id/rel_type/to_id; attachments.entity_id/property), so the PK/keyset indexes and ORDER BY are byte-ordered, matching Go''s string comparison used by fsstore/memstore. Verified: en_US.UTF-8 ordered (''A-10'',''a-2'',''A-2'',''B-1'') as ''A-10 a-2 A-2 B-1'' (diverging) vs COLLATE C ''A-10 A-2 B-1 a-2'' (= Go). Added TestListOrderIsByteWise (ordering_test.go) asserting both ListEntities and 1-at-a-time keyset pagination return Go byte order for mixed-case/punctuated IDs — the case the conformance suite (uniform T-%03d) can''t reach. Full suite green.'
status: addressed
---

## Resolution plan (fix now — merge blocker)

Force byte ordering at the schema level so the PK/keyset index, ORDER BY, and
the in-memory backends all agree:
- In `0001_init.sql`, declare key columns `COLLATE "C"`: `entities.id`,
`relations.from_id/rel_type/to_id`, `attachments.entity_id/property`.
- This makes the PK indexes byte-ordered (so keyset `>` and `ORDER BY` are
consistent and index-backed) and matches Go's string comparison exactly.
- Add a conformance/integration case with mixed-case + punctuated IDs
(e.g. `A-10`, `a-2`, `B-1`) asserting list order matches the byte-sorted
expectation, locking it in for all backends.

Bonus: `COLLATE "C"` is also faster (no locale-aware comparisons) — see leverage
note in the review.
