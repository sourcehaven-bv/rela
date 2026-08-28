---
id: AM-frontend-tests-no-network
type: automated-measure
title: The frontend unit suite cannot reach the network
description: 'noNetwork.test.ts asserts three things - that a non-default axios adapter is installed, that an unmocked GET resolves instead of attempting a connection, and that getTransforms() (the real onMounted call behind the flake) resolves. It targets the ROOT CAUSE rather than the symptom, deliberately - the symptom (an unhandled socket-hangup rejection failing an otherwise-green run) is timing-dependent and does not reproduce locally, so a symptom-shaped test would pass against the broken code. All three verified to fail (3 failed / 3) with the adapter stub removed from src/test/setup.ts, and pass with it. Catches both regressions that would reintroduce BUG-762I34: removing the stub, and adding an api module that builds its own axios instance.'
kind: test
location: frontend/src/test/noNetwork.test.ts
status: active
---
