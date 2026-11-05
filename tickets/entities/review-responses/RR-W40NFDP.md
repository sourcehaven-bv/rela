---
id: RR-W40NFDP
type: review-response
title: newLiveHub(nil) in a unit test leaves meta nil
finding: A later call to servedTargets on that hub would panic.
severity: nit
reason: The unit tests only call noteServedLocked and servedMoved; a nil meta there would fail loudly in the test that adds such a call.
status: wont-fix
---
