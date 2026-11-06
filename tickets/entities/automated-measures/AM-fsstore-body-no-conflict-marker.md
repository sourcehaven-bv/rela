---
id: AM-fsstore-body-no-conflict-marker
type: automated-measure
title: An fsstore write never produces a file that reads as conflicted
description: Every backend round-trips a body that documents conflict markers (a lone start-marker line and a fenced block); fsstore refuses to write a document that would read as conflicted.
kind: test
location: internal/store/storetest/validation.go (RoundTripsBodyDocumentingConflictMarkers); internal/store/fsstore/conflict_detection_test.go
status: active
---
