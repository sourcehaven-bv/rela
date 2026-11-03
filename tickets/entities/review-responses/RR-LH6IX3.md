---
id: RR-LH6IX3
type: review-response
title: JSON Schema should state uniqueItems
finding: 'Duplicates are refused (AC 1), so PropsSchema for a list should emit `uniqueItems: true` next to `maxItems`, so MCP clients that validate against props_schema catch it before the round trip. Add the field to PropSchema and the jsonschema test.'
severity: nit
resolution: 'PropSchema gains UniqueItems and a list emits uniqueItems: true, pinned in the jsonschema test (AC 3).'
status: addressed
---
