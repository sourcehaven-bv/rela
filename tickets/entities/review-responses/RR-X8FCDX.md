---
id: RR-X8FCDX
type: review-response
title: Same fact stated in three comments
finding: '"Expiry is not part of the identity because a refreshed token is the same principal" appears in internal/dataentry/router.go:579-583 and in jwtgate.go:202-205. "The stream ends at expiry and the write deadline is capped" appears in watcher.go:421-424, watcher.go:503-507 (sseConn.expiry) and the runSSELoop doc. commentlint duplication will flag this. Keep the identity rationale on withCredentialExpiry and the cap rationale on sseConn.expiry, and cut the field and handler comments to one line each.'
severity: nit
resolution: The identity reason lives only on withCredentialExpiry and the deadline cap reason only on sseConn.expiry; the other comments point there.
status: addressed
---
