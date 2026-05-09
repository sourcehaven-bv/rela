---
id: RR-FOLOX
type: review-response
title: Mixed-shape detection error message is non-deterministic via Go map iteration
finding: |-
    Plan says 'fail-fast 400 with offending key in pointer' but Go map iteration is randomized. Error message names whichever relation type appeared 'second'; tests flake. Recommendation: stable error code (`shape_mixed`), generic detail without naming a specific key OR sort keys lexicographically before detection. Test asserts on code only.

    From design-review: F7.
severity: significant
resolution: Plan Layer 1 specifies stable error code `shape_mixed`, generic detail ("request mixes legacy and JSON:API relation shapes; pick one"), no key naming. Test plan asserts on code only and tests both map-iteration orderings.
status: addressed
---
