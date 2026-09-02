---
id: RR-LOZBZQ
type: review-response
title: The sync regression test asserts a fact that cannot realistically break
finding: It checks *Manager's method set rather than what buildSyncEngine actually receives at runtime
severity: significant
resolution: Deleted the weak test. Replaced with TestRecordCreate_NilApplier_ErrorsNotPanics in internal/cli/sync; which drives recordCreate down the id-adoption branch with a nil applier and asserts a clean error. Confirmed it fails (nil-pointer panic) without the guard.
status: addressed
---

`internal/cli/sync_applier_assert_test.go` asserts that a typed-nil
`(*entitymanager.Manager)(nil)` held as `entityWriter` asserts to
`LocalApplier`. That is a compile-time-shaped fact about `*Manager`'s method
set: it can only fail if someone deletes a method from `Manager`, which would
break a dozen other things first.

It does **not** cover the failure mode the test's own doc describes — that the
value `buildSyncEngine` receives is something other than `*Manager` (a
decorator, a double, a future ACL wrapper). The test never calls
`buildSyncEngine`.

The right regression is: wire a `writeServices` whose `entityWriter` is not a
`*Manager` and assert `buildSyncEngine` returns an error. That only becomes
testable once the silent fallback is removed. If the typed-field fix lands,
delete the test — the compiler subsumes it.
