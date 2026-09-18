---
id: faced-route-address-parse-test
type: automated-measure
title: 'Test: every route taking an entity address parses it, exercised against a faced type'
description: 'Second occurrence of the unparsed-address shape (BUG-64MU2Q on writes, BUG-OOZBBK on relation history). A route that takes an entity address in its path must have a test that drives it with a FACED address and asserts the right row is reached — not just that the route responds. Without one the bug is invisible: it compiles, passes every bare-id test, and on fsstore/memstore even works, because their index key happens to be the state reference.'
kind: test
location: internal/dataentry/relation_history_handler_test.go
status: proposed
---

## Why

`parseEntityRef` exists and the faced-aware routes call it, but nothing forces a
new route to. A route that uses the raw path segment compiles, passes its tests,
and misbehaves only against a faced type.

Two things hide it:

- **Bare-id tests cannot see it.** Every existing relation-history test used
bare ids, so the route was fully covered and fully broken.
- **The default build masks it.** fsstore and memstore key their index on
`FormatStateRef`, so a suffixed string accidentally finds the row. Only pgstore,
which looks up by `(id, face)` columns, actually fails — so the bug does not
reproduce on the backend most tests run against.

## What it pins

For each route taking an entity address in its path, a test drives it with a
FACED address and asserts the correct row was reached — not merely that the
route returned 200. Both directions are needed: a faced address must reach the
faced row, and a bare address must reach the default one. A one-sided test
passes against a handler that ignores the face entirely.

Where the route reads something per-tail (relation history), the test fake must
key on the tail exactly as the real store does. A fake keyed on the triple alone
cannot tell a faced read from a bare one and will pass a handler that drops the
face.

## The structural fix this substitutes for

The root cause is that a bare id and an `ID@face` address are both `string`, so
the compiler cannot distinguish a parsed address from an unparsed one. A
distinct address type at the route boundary would make the mistake unavailable
rather than merely tested-for, which is the direction TKT-80EWGM took for
partial writes. That is a larger change; this measure is the cheap mitigation
until it is worth doing.

## Status

Satisfied for the relation-history route by
`TestRelationHistory_FacedAddressReadsItsOwnTail` and
`TestRelationHistory_BareAddressReadsTheDefaultTail`. Not yet audited across the
other address-taking routes — that audit is the open part of this measure.
