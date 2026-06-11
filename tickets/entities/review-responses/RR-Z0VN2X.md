---
id: RR-Z0VN2X
type: review-response
title: Benchmarks measured contracts but enforced nothing — alloc ceilings added
finding: 'Reviewer leverage finding: ns/op can''t gate CI (machine-dependent) but allocs/op is deterministic; without a gate, a regressed no-scan contract is only visible if a human runs just bench.'
severity: minor
resolution: Added TestValidateCreate_AllocCeiling (ceiling 20 vs measured ~7; an O(store) regression shows as hundreds of allocs) running in regular CI. Serial — testing.AllocsPerRun panics in parallel tests.
status: addressed
---
