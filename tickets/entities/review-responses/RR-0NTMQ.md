---
id: RR-0NTMQ
type: review-response
title: UnmarshalJSON first-byte peek doesn't cover null/string/whitespace cases
finding: |-
    Plan says peek first non-whitespace byte ([ vs {). Doesn't cover: (a) `null` as relation type value; (b) string/scalar values like `"tagged":"L-001"`; (c) whitespace/BOM handling in RawMessage; (d) how `data` absent vs `data: null` is actually distinguished. Recommendation: specify full state machine with `*json.RawMessage` triple-state, json.Decoder.DisallowUnknownFields, explicit null/scalar handling.

    From design-review: F1.
severity: critical
resolution: 'Plan Layer 1 specifies the full UnmarshalJSON state machine: trim leading whitespace; first-byte switch on `[` (legacy) / `{` (modern) / `n` (null — reject 400 `relation_value_null`) / anything else (reject 400 `relation_value_invalid`). Modern wrapper sub-state-machine handles unknown sibling keys (`unknown_field`), `data` absent (`data_required`), `data: null` (`data_required`), `data` non-array (`data_invalid_type`). Per-edge sub-unmarshal handles `meta: null` / `meta_unset: null` as absent and `meta_unset` non-string elements as 400 with index pointer.'
status: addressed
---
