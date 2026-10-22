---
id: RR-LVT21R
type: review-response
title: The retry re-read and the stale step on a missing channel are not specified or tested
finding: On ErrSessionChanged the service calls store.Head again. A cardinality-one channel deleted in between returns ErrNotFound, which must read as a nil head (unset session, implicit create), as loadHead does. A many channel must answer 404, and the plan does not say either. Conformance also lacks the case "a step with a stale expectation on a cardinality-one channel that does not exist creates nothing" (sqlchannels inserts the row in appendTarget before CheckSession, so this depends on the rollback; filechannels builds the meta in memory). Add both cases.
severity: minor
resolution: 'Plan updated: the head re-read is gone; conformance adds an invalid move on a missing cardinality-one channel that creates nothing and a missing named channel returning ErrNotFound.'
status: addressed
---
