---
id: RR-RDIQL
type: review-response
title: Per-field PATCH wire format for clearing a property is unresolved
finding: |-
    Plan Edge Cases flags this but doesn't resolve it. Verified: api_v1.go:487-491 does a pure merge (entity.Properties[k] = v) — there is no 'delete this key' operation. Sending {x: null} writes JSON-null; sending {x: ""} writes empty string. Neither is the same as 'remove the key'. For required properties this fails validation (422 → revert). For optional ones it persists a literal null/"" that breaks filters, list rendering, and Lua scripts.

    Load-bearing for daily-notes UX: ticking a checkbox 'on' then 'off' must round-trip cleanly. Need to decide and document: (a) treat empty string as unset and skip the PATCH, (b) extend PATCH with properties_unset: ["x"], or (c) accept null and have server delete. Recommend (b) — explicit, no special-casing.
severity: critical
resolution: 'Backend extension: V1UpdateEntityRequest gains `PropertiesUnset []string` field; handler iterates and calls delete(entity.Properties, k) after the merge loop. Frontend useAutoSave.scheduleUnset() routes empty/cleared optional properties through this field. AC #10 + Go test covers required-prop 422 + unknown-key tolerance.'
status: addressed
---
