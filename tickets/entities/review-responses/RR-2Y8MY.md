---
id: RR-2Y8MY
type: review-response
title: DocumentView returnTo drops query state via route.path
finding: 'frontend/src/views/DocumentView.vue uses route.path as returnTo. That strips the query, including ?from=list-id which goBack() reads. After submit, user loses the back-to-list context. Fix: use route.fullPath (and validate through the open-redirect helper). May need re-checking path-vs-fullPath semantics in isSafeReturnPath.'
severity: minor
resolution: DocumentView now passes route.fullPath as returnTo so ?from=list-id and similar query state survives the form round-trip.
status: addressed
---
