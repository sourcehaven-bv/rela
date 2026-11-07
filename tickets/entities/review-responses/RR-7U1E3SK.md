---
id: RR-7U1E3SK
type: review-response
title: 'Code: SUM overflow comment wrong for pg'
finding: 0042 claimed SUM over 2^31 rows fits a bigint; pg SUM(bigint) returns numeric.
severity: nit
resolution: 0042 now says SUM returns numeric and points to reconcile.FingerprintFromLimbSums; the int64 overflow note moved to sqlite elementDDL.
status: addressed
---
