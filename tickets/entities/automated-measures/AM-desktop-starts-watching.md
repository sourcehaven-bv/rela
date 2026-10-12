---
id: AM-desktop-starts-watching
type: automated-measure
title: Desktop wiring starts the data-entry watcher
description: A test that builds the rela-desktop data-entry app the way main.go does and asserts App.StartWatching ran, so the SSE stream carries entity:changed frames and live views on desktop.
kind: test
location: cmd/rela-desktop (planned)
status: proposed
---
