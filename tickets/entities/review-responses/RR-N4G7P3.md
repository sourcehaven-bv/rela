---
id: RR-N4G7P3
type: review-response
title: Reused fsstore keeps the boot schema's folder layout
finding: FSFactory.OpenStore captures buildSchemas(meta) (app/factory.go:73); the layout and scan read only that map (store/fsstore/layout.go:41-110). Reassembly over the old store never refreshes it, so a new type or plural is written to the wrong folder and lost on restart.
severity: critical
resolution: 'Plan changed: fsstore gains an atomic schema replacement called during the rebuild (consumer-side interface asserted at the wiring site); test adds a type with a plural, creates an entity, reopens the store and finds it.'
status: addressed
---
