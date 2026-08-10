---
id: RR-FI4DYL
type: review-response
title: 'Validation When/Then: untranspilable clause becomes forced-violation / silent-skip'
finding: 'validation.go matchFilters returns (false, err) on a CompileFilter error and has NO string fallback; caller does `if err != nil || !matches`. For a clause filter.MatchAll evaluated fine but FromFilter refuses (fuzzy-with-wildcard on a string prop, e.g. Then: title~foo*bar): OLD -> real bool, entity satisfying it -> no violation; NEW -> matchFilters returns (false,err) -> FORCED violation on every candidate (or in When:, rule silently never applies). Silent write-gating verdict flip with no diagnostic. Also diverges from automation (which string-matches the same input) — the two write-path subsystems now disagree with each other AND with the old behavior. This repo''s metamodels use only =/!=/empty so are unaffected today, but any downstream metamodel with ~/=~ in When/Then regresses. FIX: fall back to filter.MatchAll for untranspilable clauses (exact legacy verdict) OR fail loudly at metamodel LOAD (not silently per-entity at eval) with a tested decision.'
severity: critical
status: open
---
