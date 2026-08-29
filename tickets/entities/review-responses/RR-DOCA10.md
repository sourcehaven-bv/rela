---
id: RR-DOCA10
type: review-response
title: syncSeed's positional watermark and use-after-Close were unguarded
finding: syncSeed compares p.seeded >= len(seed) and applies the tail, which is sound only because seedOps is append-only — nothing enforced it, and a rewritten prefix would silently desync every later assertion. Separately, a Do after Close stood up a whole new temp project and answered from it.
severity: minor
resolution: syncSeed errors if the seed shrank and states the append-only invariant in its doc comment; the client refuses a request after Close.
status: addressed
---
