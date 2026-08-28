---
id: store-wide-case-insensitive-id-conflict-test
type: automated-measure
title: 'Test: case-variant entity IDs conflict identically across all store backends'
description: Conformance test in internal/store/storetest asserting that creating an entity whose ID differs from an existing one only by case returns store.ErrConflict in every backend. Lives in the shared harness (not a single backend package) because the property is that backends agree on entity identity; a per-backend test would let them drift apart again.
kind: test
location: internal/store/storetest conformance harness (runs for fsstore / memstore / pgstore)
status: proposed
---

## What it pins

Creating an entity whose ID differs from an existing one only by case must
return `store.ErrConflict` — in **every** backend, not just the one whose
filesystem happens to fold case.

The test lives in `storetest` (the shared conformance harness) rather than in
any single backend package, because the property being protected is that the
backends agree on entity *identity*. A per-backend test would let fsstore and
memstore drift apart again, which is the bug being fixed: entities move between
backends via migration and `rela sync`, so a project holding both `abc` and
`ABC` silently loses one on import.

## Why it is falsifying

The obvious weaker test — asserting fsstore returns an error — would pass on a
case-sensitive filesystem (Linux CI, and Go's `MemFS`) while the real bug sits
on macOS/Windows. Two things make this test actually catch the regression:

1. It runs in `storetest`, so **every** backend must satisfy it, including
the byte-exact ones (memstore, pgstore `id TEXT COLLATE "C"`). Those backends
currently accept both IDs, so they fail without a real fix — the test cannot be
satisfied by a filesystem accident.
2. It asserts the *conflict*, not the overwrite. A backend that "fixes" the
symptom by making the write idempotent would still fail.
