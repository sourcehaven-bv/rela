---
id: RR-6ZDPTQ
type: review-response
title: Pin list-export cap N source and truncation-notice placement
finding: The plan says 'capped at N' with a visible truncation notice but does not specify where N comes from or where/how the notice is injected so it survives into each output format.
severity: significant
resolution: 'v1: N is a package-level constant (e.g. 5000) in the dataentry list renderer, with a follow-up to make it per-view config (v2). The truncation notice is injected as a markdown paragraph line (''Showing N of M rows (truncated).'') appended after the table BEFORE the transform runs, so it is part of the rendered markdown for every output format. The paragraph is the contract (not a table row). Boundaries: empty list (0 rows) => valid empty table, no notice; exactly N rows => no notice; N+1 => truncated + notice.'
status: addressed
---
