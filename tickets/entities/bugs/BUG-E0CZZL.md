---
id: BUG-E0CZZL
type: bug
title: _settings allProperties order and winning definition depend on map order
description: _settings returns allProperties, allRelations and entityTypes in map order, so the order and the winning definition vary per request.
priority: low
status: backlog
---

## Description

`GET /api/v1/_settings` returns `allProperties` in Go map order
(internal/dataentry/settings_handlers.go: `for _, p := range propMap`). Two
effects:

1. The array order changes between requests, so the response is not byte-stable (found by the TKT-0M4H0S golden parity corpus, which has to sort that body to record it).
2. For a property name declared on two entity types, the first type reached decides the type, label and other fields of the entry; only `Values` are merged. Which type wins changes per process, so the settings UI can show a different property definition on each request.

Found later by the same corpus: `allRelations` and `entityTypes` in the same
response are also built in map order.

Fix: iterate entity types in sorted (or declared) order when building `propMap`,
emit `allProperties`, `allRelations` and `entityTypes` in a fixed order, and pin
it with a test that declares one property name on two types with different
types. Then drop the sort in the corpus recording.
