---
id: AM-list-property-filter-any-element
type: automated-measure
title: 'Test: list-typed properties filter by any-element match through the HTTP list endpoint'
description: 'HTTP-level table tests against /api/v1/<type> seeding a list-typed property (e.g. tags: [a, b]) and asserting eq/ne/in/contains all use any-element semantics, matching internal/filter.matchList. Every existing list-filter test in api_v1_test.go seeds only scalar string properties, which is why a filter that could never match a list property shipped unnoticed. Should also cover the multi-value wire form the FilterBar emits, plus a direct unit test for filter.matchList (currently untested).'
kind: test
location: internal/dataentry/api_v1_test.go + internal/filter/match_test.go
status: proposed
---
