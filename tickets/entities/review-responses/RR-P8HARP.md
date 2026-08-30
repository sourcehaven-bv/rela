---
id: RR-P8HARP
type: review-response
title: Both typed-nil tests were vacuous and could never have caught C1
finding: The two tests nominally covering the typed-nil path proved nothing. fakeNilVersionStore in widen_assertions_postgres_test.go silently stopped satisfying the widened interface once VersionStore() changed signature, so the resolver never took the capability branch at all — the test passed by not exercising the code. The new test in backendneutral_postgres_test.go used an untyped nil interface field, which boxes to untyped nil and never enters the guard's real path. Between them they gave full green coverage of a guard that did not work.
severity: critical
resolution: fakeNilVersionStore now returns store.VersionService holding a typed nil *pgstore.VersionStore. Added typedNilProvider + neutralVersionServiceImpl to backendneutral_postgres_test.go for the backend-neutral typed-nil branch, distinct from the existing untyped-nil subtest. Both were mutation-verified by deleting the guard and confirming each fails with its intended message.
status: addressed
---
