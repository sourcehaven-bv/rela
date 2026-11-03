---
id: RR-5OZLY4
type: review-response
title: 'Recommendation on key names: keep them'
finding: '`list: true` mirrors metamodel.PropertyDef.List (internal/metamodel/types.go:789); `max_items` matches the existing subscription config key (internal/subscription/config.go:72); `max` keeps its realtime meaning (characters) per item. `choices: {from, answer, into, other}` reads well and avoids clashing with the message type''s `post:` key. No change.'
severity: nit
resolution: 'No change: the key names list, max_items, max and choices are kept.'
status: addressed
---
