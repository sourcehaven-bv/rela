---
id: RR-BZ57ZI
type: review-response
title: Short needles fold ASCII case only
finding: Needles under three runes use LIKE, which folds ASCII only, so 'Ö' misses 'Öl'.
severity: minor
resolution: Not changed.
reason: Affects one- and two-letter searches in non-ASCII scripts only; longer needles use the trigram index, which folds Unicode.
status: deferred
---
