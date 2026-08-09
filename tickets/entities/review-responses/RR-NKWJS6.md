---
id: RR-NKWJS6
type: review-response
title: filter->predicate mapping is NOT total (fuzzy-wildcard, glob-!=, list-!=)
finding: 'Plan AC2 claims total mapping. Counterexamples: (1) fuzzy-with-wildcard (match.go:238-254) is a TWO-PHASE match (glob whole value, then trigram a length-matched prefix); predicatefns.matchFuzzy (predicatefns.go:145-154) is pure whole-string trigram — no faithful equivalent. (2) glob-on-!= (match.go:214-221) needs entity.x ~= nil and not match(entity.x,''g*'') — presence-guard+glob+negation composition untested. (3) list != = ''NONE match'' (match.go:112-117) vs contains = ''ANY match''; empty-list edge subtle. FIX: don''t claim totality — enumerate operator x type matrix, mark each cell''s predicate target, with an explicit ''unsupported -> transpiler returns error'' bucket (fuzzy-with-wildcard). Gate: every match_test.go corpus case either maps or is explicitly rejected with a clear error. A silently-different predicate is worse than a refusal.'
severity: significant
status: open
---
