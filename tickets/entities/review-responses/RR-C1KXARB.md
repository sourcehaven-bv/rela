---
id: RR-C1KXARB
type: review-response
title: --token-secret no-source error branch untested
finding: secret() returning ErrNotFound was not covered.
severity: minor
resolution: TestReconcileDiff_TokenSecret covers a project without a secrets source and asserts the flag is named and ErrNotFound wrapped.
status: addressed
---
