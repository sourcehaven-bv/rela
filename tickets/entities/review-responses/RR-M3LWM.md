---
id: RR-M3LWM
type: review-response
title: Shape-based no-op suppression defeated by realistic auto-save
finding: |-
    Plan asserts auto-save sends edges with no `meta` field when unchanged — assumes frontend dynamically omits unchanged fields. Real auto-save re-serializes full form state every debounce; every edge with meta carries `meta` in every save. Shape-based suppression triggers writes for every meta-bearing edge on every keystroke. Defeats ticket's no-op-suppression goal. Recommendation: either bring back value-based suppression (json-vs-disk float64 normalization is overstated as 'can of worms'), suppress at writeRelationCore byte-equality layer, or narrow the no-op promise scope and tell the frontend.

    From design-review: F3.
severity: critical
resolution: 'Plan Layer 2 specifies value-based no-op suppression: compute post-merge (finalProps, finalContent) and compare against current via reflect.DeepEqual. Auto-save''s main case (re-saving unchanged form with all meta fields populated) writes nothing. AC14 covers this; AC14b is the strict subset (shape-based suppression for completely absent fields). Documented assumption that JSON-vs-YAML unmarshal both produce Go-native types so reflect.DeepEqual works without explicit numeric normalization; fallback path to internal/model.PropertyValueEqual if it breaks.'
status: addressed
---
