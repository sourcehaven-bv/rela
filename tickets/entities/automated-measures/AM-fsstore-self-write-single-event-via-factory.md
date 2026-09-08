---
id: AM-fsstore-self-write-single-event-via-factory
type: automated-measure
title: A write through the production FSFactory path with the watcher running delivers exactly one store event
description: Constructs the fsstore via app.FSFactory.OpenStore (not a hand-built fsstore.Config), starts the watcher, writes an entity, and asserts one Subscribe event and one observer EntityPut. Fails if the SafeFS.OnPostWrite echo observer is not installed by production wiring.
kind: test
location: internal/app/factory_test.go TestFSFactoryWatcherSuppressesSelfEcho; internal/store/fsstore/watcher_internal_test.go TestNewInstallsSelfEchoRecorder, TestStartWatchingRefusesUnobservableFS
status: active
---
