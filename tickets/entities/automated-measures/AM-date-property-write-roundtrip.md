---
id: AM-date-property-write-roundtrip
type: automated-measure
title: A date-typed property survives a write unchanged
description: A test that writes an entity carrying a hand-authored date-typed property without touching that property, then asserts the stored value is unchanged. Pins BUG-XV7FSJ - yaml.v3 decodes an unquoted `due: 2026-08-12` to time.Time and the write path re-serializes it as RFC3339, inventing a time component the declared type does not have.
kind: test
location: internal/entitymanager/ (test to be written with the fix)
status: proposed
---

A test that writes an entity carrying a hand-authored `date`-typed property
without touching that property, then asserts the stored value is byte-identical.

Pins the defect in BUG-XV7FSJ: `yaml.v3` decodes an unquoted `due: 2026-08-12`
straight to `time.Time`, and the write path re-serializes it as RFC3339
(`2026-08-12T00:00:00Z`). The property's declared type says `date`, so the time
component is invented — and `Entity.GetString` returns "" for the `time.Time`
shape, which is why a reader that expects a string silently sees nothing.

The measure is `proposed` rather than `done` because the bug is still in
`backlog`: the test belongs with the fix, and writing it against the current
behaviour would pin the defect rather than prevent it.
