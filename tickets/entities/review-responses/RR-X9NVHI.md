---
id: RR-X9NVHI
type: review-response
title: Reader nil-handling is unspecified across 6 call sites — fail-open risk on a mis-wired runtime
finding: 'Today a nil deps.Store panics at call time in 5 of 6 sites (only markdown.go:2350 nil-guards it). The plan says ''guard like Searcher, not a panic'' but doesn''t say what a nil Reader MEANS. In a security seam the answer must be deny, not pass-through: if a wiring site forgets to set Reader, the safe outcome is ''no results / raise'', never ''fall back to RawStore''. Specify explicitly: nil Reader on a read binding raises a Lua error (or returns empty), and NEVER silently falls back to the raw handle. A fallback would turn a wiring omission into a total ACL bypass — exactly the failure mode this arc exists to prevent. Add a negative test constructing a runtime with a nil Reader.'
severity: significant
resolution: 'Specified explicitly in the plan (AC9): a nil VisibleReader on a read binding DENIES — raises a Lua error — and NEVER falls back to WritePrepStore. A wiring omission must not become an ACL bypass. Negative test constructs a runtime with a nil reader and asserts the deny.'
status: addressed
---
