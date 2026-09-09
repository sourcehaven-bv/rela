---
id: BUGA-HFSI69
type: bug-analysis-checklist
title: 'Analysis: FuzzCloneNestedValues reports correct property-key refusals as crashes'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally
- [x] Minimal reproduction steps documented
- [x] Environment/conditions noted

Reproduced by running the full sweep (`FUZZTIME=25s scripts/fuzz-all.sh`), which
failed `FuzzCloneNestedValues` on fsstore, memstore and sqlitestore. Each
failing input minimizes to a single property name:

```
string("\x00")  int(1)   # fsstore, sqlitestore — contains NUL
string("\xbc")  int(1)   # memstore — invalid UTF-8
```

Committed as seeds, so each now runs as a plain regression test: `go test
-run='^FuzzCloneNestedValues$' ./internal/store/fsstore`.

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

The target asserted `require.NoError` on `CreateEntity` for any fuzzer-chosen
property name, so a correct refusal by `storeutil.ValidateProperties` was
recorded as a crash. See why1-why5 on the bug.

## Fix Planning

- [x] Fix approach determined
- [x] Regression test planned
- [x] Related areas checked for similar issues

Approach: apply the directional oracle already used by
`FuzzPropertyValuesTypeZoo` and `createEntityOrSkip`, delegating to the exported
validator rather than assuming success.

Related areas: audited the sibling targets in
`internal/store/storetest/fuzz.go`. `FuzzPropertyValuesTypeZoo`,
`FuzzRelationKeyCollision` and `FuzzRenameKeyCollapse` already delegate to
`storeutil`; `FuzzCloneNestedValues` was the only target writing a fuzzer-chosen
property name without an oracle. The same audit found the sign bug in its
`valueType % 3`, fixed here too.
