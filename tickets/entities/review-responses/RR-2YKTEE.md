---
id: RR-2YKTEE
type: review-response
title: The character blocklist for options is narrower than the repo's existing hazard sets
finding: 'AC 1 (plan lines 108-109) refuses Cc and U+202A-202E, U+2066-2069 only. The repo already treats more as display hazards: internal/cli/printable.go:30-44 (U+061C, U+200E/F, U+2028/2029, zero-width space, word joiner, BOM, tag characters), internal/lease/lease.go:170 (adds LRM/RLM), internal/comments/comments.go:298-308 (all of Cf, variation selectors, Hangul fillers). Consequences: U+2028/2029 are not Cc, so the "single-line" guarantee (plan line 639, 773) is false; `Yes` and `Yes<U+200B>` pass the duplicate check and render as two identical buttons that post different values; tag characters smuggle hidden ASCII to agents reading the answer. Fix: refuse Cc, Cf except U+200D (keeps ZWJ emoji, which the plan wants), U+2028/2029, U+061C, U+034F and the Hangul fillers; put the predicate in one helper (e.g. next to realtimelimits or a small text package) and note the three existing lists as follow-up consolidation. Add negative cases for U+2028, U+200B and U+E0041.'
severity: significant
resolution: Display hazards move into one helper, texthazard.IsDisplayHazard, which refuses Cc, Cf except U+200D, U+2028/2029, U+034F, U+061C, variation selectors and the Hangul fillers. Negative cases for U+2028, U+200B and U+E0041 are added, and consolidating the three existing lists is noted as a follow-up.
status: addressed
---
