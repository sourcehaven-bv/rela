---
id: RR-X5JP3
type: review-response
title: validation.Service stores meta and deps.Meta as two fields that can diverge
finding: 'internal/validation/validation.go:33-41 — constructor copies deps.Meta = meta only when deps.Meta == nil. If caller passes mismatched meta and deps.Meta, the service uses s.meta for rule evaluation and s.deps.Meta for Lua-side filter lookups — silently inconsistent between Go and Lua views. Fix: either always force deps.Meta = meta (Go-side meta authoritative), or drop meta from the struct and read from deps.Meta. Don''t carry the same pointer twice with silent tiebreak.'
severity: significant
resolution: Dropped duplicate meta field from validation.Service. Constructor now unconditionally sets deps.Meta = meta (Go-side meta authoritative). All references to s.meta changed to s.deps.Meta. Go and Lua filter paths cannot silently disagree. Docstring updated.
status: addressed
---
