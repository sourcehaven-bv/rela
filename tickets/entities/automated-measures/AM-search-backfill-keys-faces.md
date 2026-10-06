---
id: AM-search-backfill-keys-faces
type: automated-measure
title: 'Test: the search backfill keys each face, and an older index is rebuilt'
description: Pins that IndexBatch keys documents as EntityPut does, so a backfilled faced entity is found in its world, and that an on-disk index without the current format version is discarded on open. Catches a write path that keys by bare id, and a key change that leaves old indexes in place (BUG-VYPK9N).
kind: test
location: internal/search/bleveindex/bleveindex_test.go
status: active
---
