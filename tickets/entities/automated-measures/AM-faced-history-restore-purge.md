---
id: AM-faced-history-restore-purge
type: automated-measure
title: History, restore and purge act on the addressed face
description: History, restore and history-purge on ID@face read and change that face's lineage only, over HTTP and CLI, on the database backends.
kind: test
location: internal/dataentry/history_face_test.go; internal/cli/history_address_test.go; internal/store/storetest/version.go (VersionsRecordTheirFace, RelationPurgeIsScopedToOneTail); e2e/tests/faces-history.spec.ts
status: active
---
