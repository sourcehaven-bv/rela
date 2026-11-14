---
id: RR-AZOV6P0
type: review-response
title: Test runtime grows
finding: readsForWith builds 8 apps per op, 4 with 1000+ padding entities.
severity: minor
resolution: Under -race the padded dataentry tests took several minutes because the search index takes each padding entity as its own write. Tests whose app keeps a search index now pad 20 against 200 (assertScaleFreeIndexed); the others keep 100 against 1,000. The on-disk flow-task fixture pads 20 against 200.
status: addressed
---
