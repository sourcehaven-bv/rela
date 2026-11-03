---
id: RR-DG6117E
type: review-response
title: 3-character rule does not predict trigram use
finding: pg_trgm extracts trigrams from runs of word characters (ctype-dependent). Needles like 'a b', '1.2', 'x-y', 'c++' or non-ASCII under C ctype can be 3+ runes with no usable trigram, giving the double-detoast cost.
severity: significant
resolution: 'Plan changed: the indexed shape is used only when the needle has a run of at least 3 ASCII letters or digits; one helper used by both builders; cases ''a b'' and ''x-y'' in tests.'
status: addressed
---
