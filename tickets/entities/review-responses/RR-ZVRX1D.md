---
id: RR-ZVRX1D
type: review-response
title: Random prime graph has no faced edges
finding: randomPrimeGraph only creates zero-face edges, so the face filter in EdgesBetween is not exercised by the equivalence test.
severity: nit
reason: The face filter is covered by the storetest FromFace cases; extending the generator is follow-up test work.
status: deferred
---
