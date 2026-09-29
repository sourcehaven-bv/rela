---
id: BUG-YC5Z07
type: bug
title: sqlite swept versions lose copy provenance (no origin columns on live rows)
description: pgstore stamps store.Origin on live entity rows (migration 0013) so the sweep can record that a version was copied from another entity. sqlitestore has no origin_* columns on entities, so swept versions of copied entities on the sqlite build show as direct edits. Same class as BUG-07DNNY.
priority: low
effort: s
why1: Swept create/update versions on sqlite carry no origin because the sweep builds VersionInput without one.
why2: The sweep captures from the live entity row, and sqlite's entities table has no origin_* columns for writes to stamp.
why3: pgstore migration 0013 added the columns to entities and entity_versions, but the sqlite port added them only to entity_versions.
why4: The origin sweep tests lived in pgstore's own test files, not in the storetest conformance suite, so sqlitestore passed every shared test without the feature.
why5: Features built on the sweep were pinned per backend rather than by contract, the same gap BUG-07DNNY found for attribution.
prevention: storetest.RunSweepOriginTests runs for every backend that declares Capabilities.Versioning and fails without the fix (AM-sweep-origin-conformance). The duplicated pgstore-only sweep tests were removed in favour of it.
status: done
---
