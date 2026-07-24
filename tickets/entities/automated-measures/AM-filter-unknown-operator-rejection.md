---
id: AM-filter-unknown-operator-rejection
type: automated-measure
title: 'Test pin: unknown/malformed filter operators are rejected with 400, never silently degraded'
description: Go tests (TestV1FilterUnknownOperator incl. the encoded `=~` case, TestV1FilterMalformedKeyRejected, and the relation-filter rejection cases) assert HTTP 400 invalid_filter for any unevaluable filter[...] param. A Vitest case asserts toApiOperator passes unknown UI operators through (with a console warning) instead of rewriting them to eq, and that `in` maps to `in`. Together they pin the loud-failure contract that replaced BUG-F1LTP1's silent degradation.
kind: test
location: internal/dataentry/api_v1_test.go, internal/dataentry/relation_filter_test.go, frontend/src/utils/filters.test.ts
status: active
---
