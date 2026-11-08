---
id: TKT-DHEA8T7
type: ticket
title: 'rela test: property-based tests with built-in properties and shrinking'
kind: enhancement
priority: medium
effort: xl
status: backlog
---

## Description

`t.property(name, generators, fn)` over schema-driven graph and operation
generators (operations taken from what `_actions` offers each principal).
Built-in properties: automation termination, validation preservation, transition
reachability. A failing case shrinks to a minimal graph and operation sequence
and is written out as a `.test.lua` regression file. Seed is printed and
`--seed` reproduces it. Needs a schema-driven generator outside
`internal/testutil`, which imports `testing`.
