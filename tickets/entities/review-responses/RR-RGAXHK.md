---
id: RR-RGAXHK
type: review-response
title: Backend semantic drift on list-valued property predicates
finding: 'propCond in internal/store/pgstore/graphquery.go compared `properties ->> key` as text for every value shape, while the naive backend used propmatch.Decide over Go values. Verified against live Postgres: an empty list `[]` renders as the two-character string "[]" (neither NULL nor blank), so is-empty returned false on pg but true on naive; and a populated list renders as JSON text `["a", "b"]`, so equality against "a" matched on naive (slices.Contains, the documented multi-select semantics) and matched NOTHING on pg. PropNotEqual correspondingly matched everything. A ''tickets not tagged blocked'' query would return the whole table on postgres and the correct subset on fsstore. Multi-select is the common ''is this field filled in?'' shape, so this would have been hit immediately. Breaks the mandatory backend-parity rule in CLAUDE.md. Not caught because the conformance suite seeded only strings, and the pgstore suite skips silently without RELA_TEST_DATABASE_URL.'
severity: critical
resolution: 'propCond now branches on jsonb_typeof: is-empty covers NULL, blank, and empty array (jsonb_array_length = 0); equality uses the containment operator `?` for arrays (with COALESCE so a missing key yields false, not NULL, keeping a surrounding NOT correct) and text comparison for scalars. Extended the conformance suite with a Props_value_shapes case covering scalars, empty lists, populated lists, ints and bools across six op/target combinations. Verified passing on both memstore (naive) and pgstore against a live PostgreSQL instance.'
status: addressed
---
