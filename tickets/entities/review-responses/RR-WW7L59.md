---
id: RR-WW7L59
type: review-response
title: Compile errors laundered into fuzz failures
finding: Any non-zero go test exit (compile error, vet, bad flag) landed in fuzz-failures.txt identically to a crashing input, producing misleading auto-filed issues.
severity: significant
resolution: Build gate before the sweep (exit 2, no issue filed); per-target output captured and classified [fuzz-crash] vs [error] in the summary; issue body explains the kinds; workflow skips issue-filing when no summary exists (setup errors).
status: addressed
---
