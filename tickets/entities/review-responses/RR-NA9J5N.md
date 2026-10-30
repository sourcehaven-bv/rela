---
id: RR-NA9J5N
type: review-response
title: pocCopiedIn is a flat list of alternating name and value
finding: 'internal/realtime/session_affordance_test.go:719-725 stores type and in: as alternating strings and walks them with i += 2. One missing element shifts every later pair. pocRealtimeDefaultIn (:737) also panics instead of failing the test. Fix: use []struct{ mt, in string }, and let pocRealtimeDefaultIn take t *testing.T and call t.Fatalf. It is called only from tests that have t.'
severity: nit
resolution: pocCopiedIn is now []struct{ mt, in string }; pocRealtimeDefaultIn takes t *testing.T and calls t.Fatalf; its three callers pass t.
status: addressed
---
