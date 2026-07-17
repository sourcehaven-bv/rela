---
id: RR-2NZRXO
type: review-response
title: Output ordering uses non-stable sort
finding: 'WhoCan sorts principals with sort.Slice (not stable). Determinism holds only while principal IDs are unique; combined with the duplicate-row bug, relative order of duplicates is unspecified and can flip between runs, defeating the stable-output goal for the drift/diff consumer. Fix: sort.SliceStable.'
severity: minor
resolution: Principal ordering now sorts a deduplicated key list (sort.Strings over unique effective-principal IDs) built from the merge map, so ordering is deterministic and there are no equal keys to reorder. Route ordering uses the total-order lessRoute comparator. Merging removed the duplicate-row source entirely.
status: addressed
---
