---
id: RR-IIGONJR
type: review-response
title: Fingerprint has no limb form for SQL sums
finding: sqlite SUM raises on int64 overflow and pg bigint is signed. Without a defined limb layout each backend would write its own carry code.
severity: significant
resolution: Fingerprint.Limbs returns four 32-bit limbs and FingerprintFromLimbSums folds per-limb sums back mod 2^128. A golden test feeds limb sums that overflow 32 bits.
status: addressed
---
