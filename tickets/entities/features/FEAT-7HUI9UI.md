---
id: FEAT-7HUI9UI
type: feature
title: Computed group keys for data-entry lists
description: Group list rows into declared sections by predicate conditions
priority: medium
status: proposed
---

## Description

A data-entry list can split its rows into sections whose membership is computed
by predicate conditions, not read from one property. Declared groups are
ordered; the first group whose condition matches a row takes it; a group without
a condition is the fallback.
