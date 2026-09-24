---
id: BUG-5IUYTI
type: bug
title: pgstore range filters use database collation; empty ~= fails 42P18
description: 'pgstore diverged from graphquerynaive on two filter shapes found by the differential harness: an empty NotEqualOrEmpty value left an unreferenced placeholder (42P18), and range filters compared under the database collation instead of byte order.'
priority: medium
effort: s
why1: propCondOn bound the property before its empty-value early return, and orderedCond compared ->> text with the column's default collation.
why2: Each backend was tested with hand-written conformance cases, which used lower-case ASCII values and never an empty NotEqualOrEmpty value.
why3: Nothing compared a SQL backend with graphquerynaive over generated inputs, so shapes no one thought to write were never run.
why4: The conformance suite checks a backend one feature at a time against expected rows a person wrote down.
why5: Backend equivalence was asserted by example rather than by comparison with the reference over generated inputs.
prevention: storetest.RunGraphDifferential (AM-graph-differential) runs random queries on awkward data against both database backends in CI.
status: done
---

## Description

The graph differential harness (TKT-B51CYD) found two pgstore divergences from
graphquerynaive:

1. A `NotEqualOrEmpty` filter with an empty value bound the property name before returning `TRUE`, leaving an unreferenced placeholder. PostgreSQL rejects the statement with 42P18.
2. Range filters (`<`, `<=`, `>`, `>=`) compared text under the database collation. The naive reference compares bytes, so `en_US.UTF-8` ordered mixed-case and punctuated values differently.
