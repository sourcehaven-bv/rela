---
id: RR-HLADU
type: review-response
title: 'fakeScriptEngine: callsSeq atomic redundant with len(calls) under mutex'
finding: internal/dataentry/document_script_test.go:65. Tests use both. Pick one — if lockless is the intent, document it; otherwise drop callsSeq and use len(calls) under the existing mutex.
severity: nit
resolution: Dropped callsSeq atomic + sync/atomic import. Added callCount() method on fakeScriptEngine that reads len(calls) under the existing mutex. Tests use fake.callCount() instead of atomic.Load().
status: addressed
---

From post-impl cranky review.

Fix: use len(f.calls) under f.mu; drop callsSeq. Simpler test code.
