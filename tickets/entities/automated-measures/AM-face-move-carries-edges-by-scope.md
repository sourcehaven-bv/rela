---
id: AM-face-move-carries-edges-by-scope
type: automated-measure
title: 'Test: face moves recover from a failed carry and keep identity edges on the entity'
description: Pins that a face move re-run finishes the edges a failed run did not move, that content-scoped edges follow the row and identity-scoped edges stay on the zero tail, and that DeleteFace on a non-last implicit face removes no edge on every backend (BUG-QEC1XJ).
kind: test
location: internal/datamigration/migrateface_tx_test.go
status: active
---
