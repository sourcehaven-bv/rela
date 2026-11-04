---
id: FEAT-LN0CT8Y
type: feature
title: Decimal property type
summary: An exact decimal metamodel property type for prices, hours and percentages, with min/max, scale, filtering, sorting and a decimal form input.
description: 'An exact decimal property type: values are stored as decimal text and compared numerically, never as binary floating point, with min/max, scale, filtering, sorting, expressions and a decimal form input.'
priority: medium
status: in-progress
---

rela has `integer` as its only numeric type. A `decimal` type holds exact
non-integer numbers (prices, hours, percentages) and behaves like `integer`
wherever integer is supported.
