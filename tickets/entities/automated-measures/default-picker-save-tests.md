---
id: default-picker-save-tests
type: automated-measure
title: Default relation picker save test coverage
description: Playwright e2e test drives the default chip/picker relation widget on an edit form, saves, and asserts via the API that the edge was persisted. Plus a Go handler test that PATCHes a relations payload to handleV1UpdateEntity and asserts the edges land in the graph / on disk. Together they cover the gap that allowed PATCH to silently drop the relations payload.
kind: test
location: frontend/e2e/forms.spec.ts, internal/dataentry/api_v1_test.go
status: active
---

## Purpose

Close the test gap that allowed `handleV1UpdateEntity` to silently drop the
`relations` payload: the default chip/picker save path had no end-to-end or
handler-level coverage (only `widget: cards` was tested, via
`relation-cards.spec.ts`).

## What it covers

- e2e: a full browser round-trip — open edit form, pick a relation target in the
default picker, click Save, assert via `/api/v1/...` that the new edge is
persisted.
- Go: `PATCH /api/v1/{plural}/{id}` with a `relations` body, assert 200, then
assert the graph contains the new outgoing edge(s) and disk has the matching
relation file(s).
