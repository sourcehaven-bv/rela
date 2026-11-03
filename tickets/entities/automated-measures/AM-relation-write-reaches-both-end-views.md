---
id: AM-relation-write-reaches-both-end-views
type: automated-measure
title: 'Regression test: a relation write reaches the views of both end types'
description: TestRelationEndTypes checks that a relation event is broadcast as entity:changed for its from and to types, deduplicated. The EntityDetail listener tests check that the detail panel reloads on entity:changed for its type, debounced, with a max wait. Catches a store event that no longer reaches the views showing either end (BUG-022MB1).
kind: test
location: internal/dataentry/watcher_relation_test.go (TestRelationEndTypes) and frontend/src/components/entity/EntityDetail.events.test.ts
status: active
---
