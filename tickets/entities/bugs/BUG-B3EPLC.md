---
id: BUG-B3EPLC
type: bug
title: TestAnalyzeProperties_StopsScanningAtCap can blow the 10-minute package budget under a loaded just ci
description: TestAnalyzeProperties_StopsScanningAtCap seeds 5000 entities into a bleve-backed fixture. Alone it takes ~66s; under the full parallel just ci on a loaded machine it ran 7m3s and blew internal/dataentry's shared 10-minute package budget, failing the whole package with a timeout panic. The package passes in 66s when given room. The cap-bounds-work assertion is sound and should stay; the seed count is the cost.
priority: medium
why1: 'internal/dataentry failed with ''panic: test timed out after 10m0s'', naming TestAnalyzeProperties_StopsScanningAtCap as still running after 7m3s.'
why2: That test seeds 5000 entities into a bleve-backed fixture. In isolation it takes ~66s; under the full parallel `just ci` on a loaded machine it inflated past 7 minutes and consumed the whole package budget.
why3: The package timeout is shared across every test in internal/dataentry, so one slow test starves the rest and the failure surfaces as a package-level panic rather than as a named slow test.
status: backlog
---

## Evidence

`just ci` on this machine:

```
panic: test timed out after 10m0s
	running tests:
		TestAnalyzeProperties_StopsScanningAtCap (7m3s)
FAIL	github.com/Sourcehaven-BV/rela/internal/dataentry	604.201s
```

The same package, run alone with room, passes:

```
ok  	github.com/Sourcehaven-BV/rela/internal/dataentry	66.132s
```

And the test alone passes in ~67s. So the work is real but bounded; the failure
is contention against a shared package budget, not a hang.

## Why this is worth fixing rather than tolerating

The failure surfaces as a package-level panic naming a goroutine dump of bleve
internals, so the first read looks like a deadlock in search rather than "one
test is slow". Found while running `/pr` for TKT-Z8K2FS, whose diff touches no
file in `internal/dataentry` at all — the time went to establishing that the
failure was unrelated.

## Fix options

The test's INTENT is sound: it asserts the cap bounds WORK, not merely output,
by counting rows the analyzer pulled (TKT-1ESTYJ). That assertion should stay.

- Lower `seeded` from 5000 to whatever still exceeds the cap by a clear margin.
The test needs "more rows than the cap", not five thousand specifically.
- Or give the package a longer `-timeout` in the justfile, which treats the
symptom and lets the next slow test hide behind it.
- Or mark it `testing.Short()`-skippable, which loses the assertion on the path
most likely to regress it.

The first is preferred: it keeps the guarantee and removes the cost.
