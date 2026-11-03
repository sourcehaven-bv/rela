---
id: RR-FDJHVA
type: review-response
title: Restricted refs carry no id; the plan renders one
finding: '§0 "Refs", AC 5 and the RlRefChip test say `{id, restricted: true}` renders the id with a "no access" tooltip. The server deliberately sends no id: internal/realtime/render.go:28 and :180 return `&RefValue{Restricted: true}` ("not even its id, which the reader may not"), and apiwire RefProp''s godoc says the same. As written, the component either always shows an empty chip, or someone "fixes" it by adding the id on the server, which leaks the ids of hidden or deleted entities (row-level rule: a hidden entity is nonexistent). Fix: a restricted ref renders a fixed placeholder ("restricted reference") with no id, no type, no link and no title. Rewrite AC 5 as "a restricted ref renders no id, type or title and no link", and test that RlRefChip never reads `id` when `restricted` is set. Make the TS type a discriminated union (`{restricted: true}` with no other fields, or `{id, type, title}`) so the compiler enforces it.'
severity: critical
resolution: A restricted ref renders a fixed "restricted reference" placeholder with no id, type, title or link, enforced by a discriminated union type and AC 5 (plan section 14 R1).
status: addressed
---
