---
id: RR-COS0LR
type: review-response
title: Nested compile error points at the outer require line
finding: The outer run re-parsed the position prefix RaiseError added and overwrote the inner module's compile-error frame.
severity: minor
resolution: run keeps an existing pending frame when the message ends with the recorded pending message. Pinned by TestRequire_NestedCompileErrorPointsAtInnerModule.
status: addressed
---
