---
id: RR-GF810PC
type: review-response
title: Picker test gaps
finding: No test for a superseded read, a non-404 error, or unmount during a read.
severity: significant
resolution: Added tests for a superseded read (aborted and ignored), a 500 (one read, 'could not load'), unmount (aborts), prefix ordering, accessible names and a relation without target types.
status: addressed
---
