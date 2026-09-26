---
id: BUG-YC5Z07
type: bug
title: sqlite swept versions lose copy provenance (no origin columns on live rows)
description: pgstore stamps store.Origin on live entity rows (migration 0013) so the sweep can record that a version was copied from another entity. sqlitestore has no origin_* columns on entities, so swept versions of copied entities on the sqlite build show as direct edits. Same class as BUG-07DNNY.
priority: low
status: backlog
---
