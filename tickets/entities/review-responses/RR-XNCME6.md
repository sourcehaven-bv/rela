---
id: RR-XNCME6
type: review-response
title: Same-entity copy read its source face raw
finding: A hidden source face passed readCopySource and a later target refusal told it apart from a missing source
severity: significant
resolution: readCopySource gates the source face through CopyReadGate before the target probe and answers ErrCopySourceMissing
status: addressed
---
