---
id: RR-14Y4TO
type: review-response
title: filechannels.Store is at the plimsoll method cap
finding: Not counting test files, filechannels.Store has 40 methods (the max-methods cap is 40) and sqlchannels.Store has 39. The plan says the counts do not change, but the backend work (apply the step, write the notice, append two lines across a segment boundary) will tempt new *Store helpers. One new method on filechannels fails the God-object lint. State in the plan that the backend helpers are free functions or methods on disk, and that the shared logic lives in package channels (PrepareSessionStep, CheckSession, ApplySession, SessionNotice, SessionAudit).
severity: minor
resolution: 'Plan updated: shared logic lives in package channels; backend helpers are free functions, methods on disk or Querier functions; no new *Store method.'
status: addressed
---
