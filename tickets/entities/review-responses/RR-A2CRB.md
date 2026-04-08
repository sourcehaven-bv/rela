---
id: RR-A2CRB
type: review-response
title: Tests mutate app.Cfg() and app.State() in place
finding: Many tests do `app.Cfg().Actions = map[...]{}` etc., scribbling on the immutable AppState pointer. This works only because Cfg() returns the same pointer production code holds. If anyone later makes Cfg() return a defensive copy, every test silently breaks.
severity: significant
reason: Pre-existing convenience in the test suite; refactor preserved the patterns to keep the diff tractable. Tracked as a test-quality cleanup ticket — should be addressed alongside the C1 fix that introduces a mutateState helper.
status: deferred
---
