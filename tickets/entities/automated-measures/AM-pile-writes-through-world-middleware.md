---
id: AM-pile-writes-through-world-middleware
type: automated-measure
title: Pile writes run through attachWorld in a test
description: Pins that pile writes refuse ?world= through the real middleware and accept a face the default world hides when written without one. The SPA test pins that writes from a world-bound page send no world.
kind: test
location: internal/dataentry/piles_handler_test.go TestPiles_WriteThroughWorldMiddleware; frontend/src/composables/usePiles.test.ts
status: active
---
