---
id: RR-4YNQ1PZ
type: review-response
title: Lint fails in reconcile
finding: golangci-lint reports mnd (magic number 32) twice in Fingerprint.Limbs and gocritic truncateCmp in TestLimbs_RoundTrip.
severity: significant
resolution: Added const limbBits used by Limbs and FingerprintFromLimbSums; the test compares widened values.
status: addressed
---
