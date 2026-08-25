---
id: RR-XGJLO1
type: review-response
title: Compile-time assertion wrapped in a func Test, and unused *testing.T in newFakeStore
finding: TestPgstoreSatisfiesWidenedInterfaces is a compile-time assertion wrapped in a test function with a t.Log to justify it — it cannot fail at runtime, so it only adds a fake green line to the report. Separately, newFakeStore takes *testing.T and calls t.Helper() but never registers cleanup; taking a T and not using it for t.Cleanup is a smell even where harmless.
severity: minor
resolution: Moved the assertions to file scope as `var _ userStateProvider = (*pgstore.Store)(nil)` etc., matching the precedent in version_store.go:40-48 — same guarantee, build-time failure, no fake test. Added t.Cleanup(func() { _ = m.Close() }) to newFakeStore.
status: addressed
---
