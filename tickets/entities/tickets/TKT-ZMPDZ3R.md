---
id: TKT-ZMPDZ3R
type: ticket
title: AtomicWrites capability flag and capabilities.atomic_batch on _schema
kind: enhancement
priority: medium
effort: s
started: "2026-11-04"
completed: "2026-11-04"
status: done
---

## Description

Decide at wiring whether the deployment's store rolls a transaction back
(sqlitestore, pgstore) and pass that as an `AtomicWrites` flag to the data-entry
app. Advertise it as `capabilities.atomic_batch` on `GET /api/v1/_schema`
(DEC-ODU65KR item 1). The flag is derived from the assembled store, not the
build tag, so a sqlite build that falls back to markdown files reports false.
