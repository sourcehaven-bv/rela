---
id: RR-03H3RH
type: review-response
title: Ring resync and lag reveal hidden write volume
finding: Ring overflow and buffer lag depend on the total number of writes across all types, so a principal can estimate write volume on types it cannot read (breaks AC5 and docs/acl-security.md).
severity: significant
resolution: Replaced the event ring with per-type last-seq state. Resync depends only on token validity, nothing lags, and frame timing depends only on readable types and the fixed ping timer.
status: addressed
---
