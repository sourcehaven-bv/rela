---
id: RR-YADMI
type: review-response
title: TestConcurrentReadDuringOnReload too weak
finding: Only checks state.Load() returns non-nil with non-nil Cfg/Meta. Does not check any cross-field invariant or exercise any handler. A reader observing a torn handler response would not be caught.
severity: minor
resolution: 'TestConcurrentReadDuringOnReload now bumps to 8 readers and 200ms duration, and asserts a cross-field invariant: a published AppState always has at least as many StyleMap entries as metamodel.Types. A torn publish that put a new metamodel into the snapshot before the matching StyleMap would fail this assertion under -race. A more thorough handler-level test (TestHandlerCoherenceUnderReload) is tracked as TKT-7DJ2O.'
status: addressed
---
