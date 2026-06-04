---
id: RR-889RK
type: review-response
title: Search Backend returns IDs only; pg backend must NOT filter/sort
finding: 'The plan said the pg search backend would do tsvector+trgm and ''return ranked IDs'', but the real Service contract (index.go:30-70) is narrower: Service always calls backend.Search(text, 0) (limit 0 = all), then the SERVICE loads each entity from the reader and applies Types, PropertyFilters, and Limit itself. The Backend''s ONLY job is text->[]ID, case-insensitively matching ID + content + string properties (mirroring MatchText in filter.go:70-84). If the pg backend tries to apply filters/types/limit it will double-filter and diverge from conformance. Also: when q.Text=='''' the Service uses listAll() and never calls the backend at all — so empty-query-lists-all does NOT depend on the search backend.'
severity: significant
resolution: 'Incorporated into the plan (PLAN-LUXFP) and implemented: pgstore''s SearchBackend.Search returns entity IDs only (case-insensitive substring over search_text); the search.Service applies type/property filters and the limit on top. Verified by RunSearchTests passing in the conformance suite (commit 296c5f3f).'
status: addressed
---

## Resolution (plan update)

- pg search `Backend.Search(text, limit)` returns **entity IDs only**, ranked best-effort; it ignores filters/types/sort (Service applies those).
- It must match the **same fields** LinearSearch/MatchText do: case-insensitive substring across ID, content, and string-valued properties — the 21 `RunSearchTests` assertions are written against that semantics (e.g. "login" matches title AND content => 2 hits).
- `limit` arrives as 0 from the Service; return all matches (or a sane high cap).
- The fuzzy/wildcard *parity* concern (bleve fuzziness=1, `*`/`?`) is real for the CLI/API search UX but is **not** exercised by the conformance suite (which only does substring). Keep tsvector/trgm but ensure plain substring queries still satisfy MatchText semantics. Document any divergence.
