---
id: AM-analyze-fails-closed-on-unreadable-entity
type: automated-measure
title: "`rela validate` exits non-zero when a scan could not read every entity"
kind: test
location: internal/analysis/ + internal/cli/ (tests to be written with the fix)
status: proposed
description: "A project containing one unparseable entity file must make `rela validate` exit non-zero and name the unreadable file, never print 'All validations passed.'. Pins BUG-4KPN2M, where a partial scan was reported as a clean run and hid a real violation from CI."
---

Pins BUG-4KPN2M.

A fixture project containing **one** unparseable entity file must make
`rela validate`:

1. exit **non-zero**,
2. name the file it could not read, and
3. **not** print `All validations passed.`

Must **fail on current `develop`**, where `collectEntities` logs a warning,
returns the partial slice, and validation reports success over whatever
survived.

The sharp variant — the one that reproduces the real incident — places a
genuine rule violation on an entity that is *not* the malformed one, and
asserts it is still caught. That is what actually happened: BUG-E9DYW5 was
`done` with no `has-review`, and the rule never fired because a *different*
file (its linked measure) failed to parse.

Companion assertions belonging in the same suite:

- A healthy project exits 0 with byte-identical output, so failing closed does
  not become failing noisy.
- An injected iterator error propagates from each `collectEntities` call site
  rather than yielding a short slice — nine sites today, plus the two
  warn-and-skip paths in `FindOrphansWithScope`.

This measure guards a **silent** failure mode: the run reports success, so
without an explicit assertion on the exit code nothing in CI can notice the
regression. That is precisely how the original violation survived.
