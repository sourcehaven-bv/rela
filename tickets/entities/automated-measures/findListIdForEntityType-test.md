---
id: findListIdForEntityType-test
type: automated-measure
title: findListIdForEntityType getter has unit test
description: Unit test for the schema store getter used by EntityDetail for post-delete navigation. Guards against regressions that would reintroduce the broken /list/{entityType}s assumption.
kind: test
location: frontend/src/stores/schema.test.ts
status: active
---
