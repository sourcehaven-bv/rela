---
id: dirty-registry-async-unmount-test
type: automated-measure
title: 'Regression test: dirty-registry unregisters when registration happens after an await in onMounted'
description: Vitest component test mounting a harness that replicates DynamicForm's lifecycle (registration inside async onMounted after an await, cleanup via a synchronously registered top-level onBeforeUnmount) and asserting the registry empties on unmount.
kind: test
location: frontend/src/components/forms/dirtyFormRegistry.test.ts
status: active
---
