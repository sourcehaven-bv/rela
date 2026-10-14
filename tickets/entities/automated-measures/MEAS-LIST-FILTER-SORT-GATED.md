---
id: MEAS-LIST-FILTER-SORT-GATED
type: automated-measure
title: List filter and sort refuse hidden fields
description: 'TestListFieldGate_HiddenPropertyRejected sends filter[] with every operator (eq, ne, contains, in, lt, lte, gt, gte, array form) and sort (asc, desc, secondary key) on a field visible: hides from the principal and expects 400 property_not_visible naming the field and no value. Companion tests cover when:-conditional visibility, the shared pipeline, list q= and the gated-query guard.'
kind: test
location: internal/dataentry/list_field_gate_test.go
status: active
---
