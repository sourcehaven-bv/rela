---
id: RR-S45X2X
type: review-response
title: 'rela-tickets cache step ungated: empty hashFiles collapses the key onto its own restore-key prefix'
finding: In .github/workflows/ci.yml the rela-tickets job gates its checkout and Set up Go on steps.gate.outputs.applies == 'true', but the added `Cache Go build cache` step was not gated. On an ungated run there is no checkout, so hashFiles('**/go.sum') returns the empty string rather than failing. The key then degrades to `go-rela-tickets-<os>-<arch>-<ver>-`, which is byte-identical to the restore-keys prefix. That saves a cache built from an empty workspace under the very key every later run falls back to, permanently poisoning the fallback the restore-keys exist to provide.
severity: critical
resolution: 'Added `if: steps.gate.outputs.applies == ''true''` to the cache step, matching the checkout, with a comment explaining the empty-hashFiles failure mode. Also audited the whole workflow for the general pattern (conditional checkout + ungated cache) via a YAML scan; rela-tickets was the only site.'
status: addressed
---
