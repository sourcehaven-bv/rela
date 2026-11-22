---
id: RR-GZGCNGS
type: review-response
title: '[security] Scan-cap cursor is the seq of a hidden record'
finding: '[security] Past the scan cap the cursor names a record position the reader was not shown.'
severity: minor
reason: 'Accepted: it tells only that some hidden record exists at a position. Documented in the ACL guide and godoc.'
status: wont-fix
---

[security] Past the scan cap the cursor names a record position the reader was
not shown.
