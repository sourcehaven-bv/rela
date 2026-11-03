---
id: RR-BDTKNT
type: review-response
title: BenchmarkOpen does not report allocations
finding: 'internal/store/fsstore/index_scan_test.go:139-165: the benchmark documents open cost, but the scan''s cost is mostly the index maps and the sorted order slices, and it reports only ns/op. Add b.ReportAllocs() before the loop at :161, so a regression that turns the scan into per-file reads or stats shows up in allocs/op as well as in wall time, which is noisy across machines.'
severity: nit
resolution: BenchmarkOpen calls b.ReportAllocs().
status: addressed
---
