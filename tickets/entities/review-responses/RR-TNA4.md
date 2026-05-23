---
id: RR-TNA4
type: review-response
title: Renderer.checkbox assigned via direct property mutation is undocumented contract
finding: 'marked''s documented extension point is marked.use({ renderer: { checkbox: ... } }). Direct property mutation works but is undocumented contract — a future marked version that makes Renderer have private fields, switches to readonly, or relies on prototype-chain dispatch could break this silently.'
severity: nit
resolution: 'Switched from direct property mutation on Renderer instance to the documented `new Marked({ renderer: { checkbox(...) { ... } } })` API. Per-render Marked instance preserves the closure over `cbIdx`, so each render still gets its own counter.'
status: addressed
---
