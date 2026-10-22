---
id: RR-9KM3JO
type: review-response
title: docscapture always uses the Nop resolver
finding: internal/docscapture/server.go passes NopFieldVerdictResolver{}, so screenshots of a ceiling-restricted role show fields the server hides.
severity: minor
reason: Pre-existing, not a disclosure (seeded temp project for screenshots only), and outside this bug.
status: deferred
---
