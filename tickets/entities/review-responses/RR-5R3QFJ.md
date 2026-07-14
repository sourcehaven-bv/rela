---
id: RR-5R3QFJ
type: review-response
title: Mixed date/datetime sort degrades to lexical; datetime = date-operand semantics undefined
finding: 'sort.go comparePropValues only compares by value when both props share a type string. date and datetime differ, so a mixed-column sort falls to typeRank then compareStrings (lexical, not chronological). Fix: datetime shares typeRankDate (so date+datetime interleave chronologically) AND the cross-type path must run toTime on both. Separately, matchDate uses .Equal (instant-equal to the second): a filter `due=2026-07-13` vs a stored `2026-07-13T12:30:00Z` parses operand as midnight and never matches. Decide + document: is datetime `=` strict-instant or day-granular? Recommend documenting strict-instant and steering users to `>=`/`<` for ranges, or add day-granular `=` for datetime.'
severity: significant
resolution: 'Accepted into plan. (1) datetime shares typeRankDate (NOT a separate rank) so date+datetime interleave chronologically in mixed sorts; verify the cross-type compare path in comparePropValues runs toTime on both when one side is date and the other datetime (may need to treat date/datetime as same-type for the equality gate). (2) Document datetime `=` as STRICT-INSTANT (second-granular): `due=2026-07-13` (midnight) will not match `2026-07-13T12:30:00Z`; steer users to `>=`/`<` ranges for day queries. Add match_test + sort_test cases covering mixed date/datetime and strict-instant equality.'
status: addressed
---
