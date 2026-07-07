---
id: RR-VQD8FN
type: review-response
title: Single-select (max_incoming=1) incoming picker untested on create
finding: The empty-baseline fix also feeds the single-select branch (selectEntity replaces list with [id] when !isMulti). Both new unit tests used max_incoming=10; the single-cardinality incoming path on create was uncovered.
severity: nit
resolution: 'Parameterized the test schema with maxIncoming and added ''create mode: single-select incoming picker (max_incoming=1) emits the addition''.'
status: addressed
---
