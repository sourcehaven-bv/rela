---
id: RR-XVC7I4
type: review-response
title: Per-row verb cost is not measured
finding: roleFor allocates filtered slices when a client ceiling is active, roles x dimensions per row. No graph I/O, but pin the cost with a benchmark that has a ceiling and several roles, and compute the field and option universes once per call.
severity: minor
resolution: 'Added BenchmarkFieldVerdicts_RoleUnion (all roles, client ceiling, every dimension closed): about 8 us and 72 allocs per row. BenchmarkVerdicts is unchanged against develop (about 10 us, 116 vs 117 allocs).'
status: addressed
---
