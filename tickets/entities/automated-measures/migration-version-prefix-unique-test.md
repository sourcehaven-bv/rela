---
id: migration-version-prefix-unique-test
type: automated-measure
title: 'Test: pgstore migration version prefixes are unique (no new duplicates)'
description: Loads the embedded migration set and fails if a NEW file introduces a duplicate integer version prefix (the pre-existing 0003 pair is grandfathered). Prevents recurrence of BUG-TY2XQC where a second file at an already-applied version is silently skipped.
kind: test
location: internal/store/pgstore/migrate_load_test.go:TestLoadMigrationsIntegrity
status: active
---
