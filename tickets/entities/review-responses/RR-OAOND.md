---
id: RR-OAOND
type: review-response
title: relationsEqual doesn't deep-walk; uses fmt.Sprintf surface comparison
finding: |-
    Plan decision #11 calls for 'edge whose target + final-meta + final-content **deep-equal** the current state'. Implementation uses `fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)` which works for flat maps via Go's key-sorted %v map formatting but is fragile for: nested slices of mixed types, deeply nested maps inside slices, the int-vs-string false-positive (C4).

    Fix: replace propsValueEqual with reflect.DeepEqual in relationsEqual. Same one-liner fix as C4 but applied at the relationsEqual level too.
severity: significant
resolution: Same as RR-LMVF7 — relationsEqual now uses valueEqual which uses reflect.DeepEqual with numeric normalization. No more fmt.Sprintf surface comparison.
status: addressed
---
