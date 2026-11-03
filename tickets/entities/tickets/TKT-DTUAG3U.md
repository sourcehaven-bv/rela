---
id: TKT-DTUAG3U
type: ticket
title: Add a decimal property type
kind: enhancement
priority: medium
effort: l
started: "2026-11-03"
status: review
---

## Description

Add `type: decimal` to the metamodel. It holds exact non-integer numbers
(prices, hours, percentages) and behaves like `integer` wherever integer is
supported: `min`/`max`, `list`, `required`, `default`, validation, filtering and
sorting with numeric order, predicates, computed properties, roll-ups, the
data-entry number input and `display: currency`.

Values are parsed and compared exactly (math/big.Rat), never as float64. A new
`scale` option caps the digits after the point.
