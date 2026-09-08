---
id: RR-SR2NS0
type: review-response
title: visibleHeaderIDs dropped the face half of the gate filterVisible applied
finding: 'Security review: the header-based neighbour gate applied PermitsReadMany only, not faceReadable. Not exploitable today because the query is default-face-only, but a latent under-check once a caller adds a World or AllStates.'
severity: minor
resolution: visibleHeaderIDs applies faceReadable per candidate exactly as filterVisible does; headers carry Face so it costs nothing.
status: addressed
---
