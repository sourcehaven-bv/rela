---
id: RR-U081VJ9
type: review-response
title: Adversarial rationale for 128 bits is wrong
finding: The Wagner k-tree attack trades crafted rows for work at any width; no width resists a writer. The row counts in the plan were wrong.
severity: minor
resolution: Kept N=128 for a 2^-128 accidental match rate. The package doc says the fingerprint detects accidental difference only and that a writer in scope can hide a change from sync. No cost claim.
status: addressed
---
