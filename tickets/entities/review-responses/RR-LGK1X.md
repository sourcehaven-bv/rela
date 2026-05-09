---
id: RR-LGK1X
type: review-response
title: null inside JSON for meta / meta_unset / meta_unset elements unspecified
finding: |-
    Plan handles `content: null`. Doesn't specify: `meta: null` (clear or absent?), `meta_unset: null` (absent?), `meta_unset: ["x", null]` (will fail with bad Go error 400). Recommendation: explicit edge-case rows. `meta: null` → absent. `meta_unset: null` → absent. `meta_unset` containing null/non-string element → 422 with index pointer. Validate at unmarshal.

    From design-review: F8.
severity: significant
resolution: 'Plan to specify in the rewrite: meta: null → treated as absent. meta_unset: null → treated as absent. meta_unset containing a null/non-string element → hard 400 wire-format error (caller bug, not a soft data condition). These are wire-shape issues, not data-state issues, so they stay 400.'
status: addressed
---
