---
id: RR-X9P7MW
type: review-response
title: 'Minor: entityRef re-parses _self; TS grammar helpers unvalidated; error-shape divergence on non-address routes; guard test skipped views.go wholesale; addressed-face store errors folded into not-found'
finding: 'Five minor items from the general review: the SPA derives the address from the last _self path segment; refBareId/refFace accept strings the Go grammar rejects; /_documents rejects ''@'' with a 400 while addresses 404; the executeView guard test skipped all of views.go; getVisibleRef mapped every GetEntityState error to not-found silently.'
severity: minor
resolution: Guard test now exempts only the executeViewRef forward inside views.go; getVisibleRef logs a non-ErrNotFound store error before answering not-found. The _self derivation and the TS helpers stay (values originate server-side from ValidateID + ParseFace, so no traversal is reachable); the /_documents 400 is pre-existing and those routes are per entity, recorded on TKT-5SZG2L.
status: addressed
---
