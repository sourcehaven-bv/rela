---
id: RR-M0LIU
type: review-response
title: 'No test for backward compatibility: existing `id_prefix` singular field must keep working'
finding: 'The plan adds `id_prefixes` but says ''keep `id_prefix` for backward compat''. No explicit test was listed that verifies a single-prefix type still has `id_prefix` populated in the JSON response after the change. Add: ''Go test TestV1Schema_SinglePrefix_Compat asserts that a type with id_prefix: TKT- returns both id_prefix: TKT- AND id_prefixes: [TKT-]''.'
severity: minor
resolution: AC4 now explicitly lists TestV1Schema_SinglePrefix_Compat asserting both id_prefix and id_prefixes populated for single-prefix types.
status: addressed
---
