---
id: AM-contract-sync-on-marker
type: automated-measure
title: Contract test syncs with the server on a marker event
description: The fake-vs-server contract test takes its quiet points from the flush that reports a marker write instead of from a delay; it fails when an entity:changed lands in the wrong window.
kind: test
location: frontend/contract/fakeVsServer.contract.test.ts
status: active
---
