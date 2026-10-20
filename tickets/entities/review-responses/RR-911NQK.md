---
id: RR-911NQK
type: review-response
title: Leeway coupling is prose-only
finding: internal/jwtauth/verifier.go:184-190 says a future leeway must also be added to ExpiresAt, or streams opened inside the leeway reconnect in a loop. Nothing enforces this. Someone adding jwt.WithLeeway at verifier.go:99-102 or 312-315 will not read a field comment 80 lines further down. Define one leeway value (even if it is 0), pass it to jwt.WithLeeway, and compute ExpiresAt as exp plus that value in VerifyAssertion. The comment then becomes unnecessary.
severity: minor
resolution: jwtauth has one assertionLeeway constant passed to jwt.WithLeeway, and ExpiresAt is exp plus that leeway; TestVerifyAssertion_ExpiryIncludesLeeway pins it with a one-minute leeway.
status: addressed
---
