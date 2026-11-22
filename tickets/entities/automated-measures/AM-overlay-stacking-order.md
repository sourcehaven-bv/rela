---
id: AM-overlay-stacking-order
type: automated-measure
title: Overlay z-order as the bundle resolves it
description: A unit test resolves the --rl-z tokens across every stylesheet main.ts imports, in import order, and asserts session screen > confirm > dialog and the library's internal order.
kind: test
location: frontend/src/styles/rlStacking.test.ts
status: active
---

Fails when a later stylesheet overrides the rebased z-scale, or when a dialog,
confirm or the session screen leaves its tier.
