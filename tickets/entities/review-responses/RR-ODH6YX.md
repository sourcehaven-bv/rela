---
id: RR-ODH6YX
type: review-response
title: Trace JSON schema and nested-child resolution were untested
severity: significant
status: addressed
finding: The new TestWriteTrace_TitleResolver covered only a flat resolver-set + nil case. Nothing pinned the trace JSON field set (so the Properties bloat slipped in with no test signal), and nothing proved resolution applies to nested child nodes (writeTraceNode recurses).
resolution: Strengthened TestWriteTraceJSON to assert Properties (and a canary value) are absent from the trace JSON — pins the raw-JSON contract. Added a "resolver applies to nested child nodes" subtest using a per-ID idTitleResolver, asserting both root and child titles resolve, proving recursion.
---
