---
id: BUG-UJ5THN
type: bug
title: Stale identity assertion in policy-less script-read seam test reddens CI on every open PR
description: TestScriptReadSeam_PolicylessProjectStaysUnrestricted fails on develop at HEAD, turning the Test job red on all 9 approved dependabot PRs (#1219-#1227) and every other open PR. Production behaviour is correct; only the test assertion is stale.
priority: high
effort: xs
why1: The test asserted pointer identity between the policy-less script reader and app.store (any(reader) != any(app.store)), which no longer holds.
why2: 'PR #1208 (TKT-1WV50C) changed App.scriptReader''s policy-less branch from `return a.store` to `return visibility.Unrestricted(a.store)`, so the reader is now a *visibility.UnrestrictedReader wrapper rather than the store itself.'
why3: The test pinned an implementation detail (which object is returned) instead of the contract it exists to protect (policy-less means no gate and no redaction), so a deliberate, behaviour-preserving refactor broke it.
why4: '#1208 was a rename/legibility refactor whose author updated the production wiring and the visibility package''s own tests, but did not run or notice the dataentry package''s fail-closed test that depended on the old return identity.'
why5: Tests that assert on object identity rather than observable behaviour create hidden coupling to refactorable internals; nothing in review or tooling flags an identity assertion as the fragile form when the safer contract assertion is available.
prevention: Assert the ungated contract (type is *visibility.UnrestrictedReader plus a read that passes through to the store) rather than pointer identity with the store. More generally, when a seam test exists to pin a security contract, assert the observable behaviour of that contract so a legibility refactor of the wiring cannot redden CI while behaviour is unchanged.
status: done
---

## Symptom

`go test ./internal/dataentry/` fails on `develop` at HEAD:

```
failclosed_test.go:137: policy-less scriptReader should be the raw store, got *visibility.UnrestrictedReader
--- FAIL: TestScriptReadSeam_PolicylessProjectStaysUnrestricted (0.00s)
```

Because the `Test` job runs on every PR, this single failure blocked all 9
approved dependabot PRs plus the open feature/docs PRs.

## Root cause

#1208 (TKT-1WV50C) named the ungated read path:

```go
-	return a.store
+	return visibility.Unrestricted(a.store)
```

The test's identity assertion (`any(reader) != any(app.store)`) predates that
change. The `scriptTracer` half still returns `a.tracer` directly, so its
identity assertion remains valid and is left untouched.

## Fix

Pin the contract instead of the identity: assert the reader is a
`*visibility.UnrestrictedReader` and that a read passes straight through to the
store. This keeps the test meaningful against the current wiring rather than
re-coupling it to the detail #1208 deliberately replaced.

Fixed in PR #1228.
