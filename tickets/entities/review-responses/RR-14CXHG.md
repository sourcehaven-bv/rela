---
id: RR-14CXHG
type: review-response
title: Docs table implies a meaningful 403-vs-denied asymmetry between header/env and JWT rows
finding: 'The ''Reserved system: identities'' table in GUIDE-server-security says header/env yield ''403 on /api/, logged'' while the verified-JWT row says ''denied, logged''. The JWT path actually returns 401 via the gate''s uniform deny. Accurate as written, but a reader comparing the rows will assume the asymmetry encodes something intentional about the reserved check rather than just the gate''s existing uniform-401 behaviour. One clause noting the gate answers 401 uniformly -- so a reserved subject is indistinguishable from any other failed assertion -- removes the ambiguity.'
severity: nit
resolution: 'Added a clause after the table in GUIDE-server-security: the asymmetry is not about the reserved check -- the JWT gate answers 401 uniformly for every failed assertion, so a reserved subject is indistinguishable from an expired or unsigned one, and the distinction lives in the log rather than the response. Docs regenerated from the docs-project source.'
status: addressed
---
