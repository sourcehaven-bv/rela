---
id: RR-V5FXGI
type: review-response
title: Per-identity watch count must be atomic
finding: 'Security design review: checking the count separately from registering lets concurrent calls exceed the cap.'
severity: nit
resolution: Checked and registered under one mutex, released in a defer; AC7 starts the over-cap watch concurrently.
status: addressed
---
