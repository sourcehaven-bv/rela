---
id: scope-position-ordering-parity-test
type: automated-measure
title: 'Test: _position observes the same ordered set as the list endpoint'
description: 'Backend tests for the scope-position endpoint. TestV1PositionMatchesListOrdering asserts that `/api/v1/_position` resolves position over the exact same filtered/sorted set the list endpoint produces for a given scope, so the two pipelines cannot silently diverge (the original BUG-4GEC9 divergence came from a parallel client-side reimplementation gated on a per_page cap). TestV1Position covers middle/first/last/filtered/search/404; TestV1PositionBadRequest covers strict-decoder rejections (missing id/scope, malformed JSON, unknown source/type, bad filter key). Frontend useScopeNavigation.test.ts covers descriptor construction including source=search for q.'
kind: test
location: internal/dataentry/scope_test.go (TestV1Position, TestV1PositionBadRequest, TestV1PositionMatchesListOrdering) + frontend/src/composables/useScopeNavigation.test.ts
status: active
---
