---
id: RR-Y5SU7NV
type: review-response
title: Benchmark gate turns off silently when no row pairs
finding: benchstat cannot pair rows whose GOMAXPROCS suffix differs between runner core counts (BenchmarkX-4 vs -8), or after a rename; ParseBenchstatCSV then returns no rows and the gate passes without saying so. benchstat@latest could also change its CSV.
severity: significant
resolution: 'Addressed: go test pins -cpu 4; check writes a report note when no benchmark appears in both runs; benchstat is pinned to v0.0.0-20261009192801-be2c69fb417e.'
status: addressed
---
