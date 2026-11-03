---
id: AM-released-migrations-immutable
type: automated-measure
title: 'Test: released pgstore migrations are unchanged'
description: Pins a sha256 of every pgstore migration that shipped in a release and fails if one is edited, because the migrator tracks only version numbers and an edited released migration never reaches existing databases. Prevents recurrence of BUG-DQLE0K.
kind: test
location: internal/store/pgstore/migrate_load_test.go:TestReleasedMigrationsUnchanged
status: active
---
