---
id: RR-NX0F
type: review-response
title: gitCryptHeader constant duplicated in test files — two-source-of-truth
finding: 'internal/store/fsstore/gitcrypt.go:8 defines gitCryptMagic; gitcrypt_integration_test.go:17 redefines gitCryptHeader independently as a separate byte slice. The integration test is in `_test` package so cannot reference the lowercase identifier directly. If someone changes the magic header (e.g. a future git-crypt v2), one of the two will be wrong silently. Fix: export an internal test helper, or add a unit test in gitcrypt_test.go (same package) that asserts bytes.Equal between the two — forces noisy failure on divergence.'
severity: minor
status: open
---
