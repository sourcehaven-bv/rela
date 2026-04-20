---
id: RR-ZANS2
type: review-response
title: Deprecated re-exports in crypto_verify.go are used only by in-package tests
finding: 'crypto_verify.go:13,19 re-exports the two integrity errors with Deprecated: docs. Only callers are internal/store/fsstore/crypto_test.go:223,252 — same package. Nobody outside fsstore imports these. Not doing migration work; preserving an internal symbol that could import integrity directly.'
severity: minor
resolution: Deleted the two Deprecated re-exports from crypto_verify.go and updated crypto_test.go to import integrity.Err* directly. Zero external callers, so the re-exports were dead weight.
status: addressed
---
