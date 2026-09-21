---
id: RR-Y5H1SO
type: review-response
title: SQL/index expression equivalence held by coincidence, not by test
finding: 'Code review S4. TestOrderSQL_IndexRankAppearsVerbatimInQuery asserted the invariant for ONE hard-coded input ({todo,doing,blocked,done} on "status"). The reviewer probed quotes, empty strings, unicode and single-element lists and found they all match - so the property held, but was not pinned. A quoting or separator change that only bites on an unusual value would ship an index that is built, maintained on every write, and never used, with no failing test. The EXPLAIN test cannot compensate: it documents that plan_cache_mode governs cached plans only, so a one-shot EXPLAIN substitutes parameters and finds the index either way.'
severity: significant
resolution: 'Converted to a table over ten shapes an operator could actually write: single value, embedded quote, empty string declared, unicode and emoji, spaces and punctuation, SQL-looking text, a duplicate value, a quote in the PROPERTY name, and a descending spec. Added FuzzOrderSQLRankMatchesIndex over arbitrary (property, values) input, since the invariant - the index expression appears verbatim in the query expression - is exactly the shape fuzzing is for, and the repo already fuzzes the store conformance suite. Ran 450,000 executions in 26s with no divergence. That converts ''currently true'' into a proven property, which was the reviewer''s actual ask.'
status: addressed
---
