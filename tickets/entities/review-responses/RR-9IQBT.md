---
id: RR-9IQBT
type: review-response
title: Coercion/equality semantics undefined for the common case (form values are strings vs typed literals)
finding: 'The spec says ''permissive, filter-style coercion'' but does not define the actual comparison table. This is the crux of correctness: a checkbox value in formData may be a JS boolean `true`, but the literal in `form.has_processors == true` parses as a boolean; an enum value is a string; a number field may arrive as a string from an input. What does `form.count == 3` do when formData.count is the string ''3''? What does `== true` do against ''true'' vs true vs ''on''? Go predicate uses strict Lua equality (cross-type compares are compile errors); the JS engine explicitly rejects that in favor of coercion — so it MUST specify its own table, or authors get silent false-negatives. Required: a concrete coercion/equality matrix (string<->number, string<->bool, nil/unset handling) with tests, and a note on how it intentionally differs from predicate''s strict equality (a real divergence from the ''congruent'' claim that must be documented, not glossed).'
severity: significant
resolution: 'Added a concrete coercion/equality table to the spec: nil/unset handling, bool-literal coercion (accepts JS bool or ''true''/''false'' string), number coercion via Number(), string byte-compare, ordered numeric-else-lexicographic, regex with invalid->false. Explicitly documented as permissive and divergent from predicate''s strict equality. AC3 requires a test per row.'
status: addressed
---
