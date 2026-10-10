---
id: TKT-W833MJ
type: ticket
title: 'Configure space: migration history view'
kind: enhancement
priority: low
effort: m
status: backlog
---

## Description

Split from TKT-F5NGMG (acceptance criterion 12). The Configure space shows no
history of applied data migrations.

Add a migration history screen to the Configure space. It lists applied
migrations with date, origin (CLI or Configure save), author and records
changed. Opening one shows its steps.

This needs a read endpoint over the per-store `datamigration.StateStore` (the
applied list) plus the migration files in `migrations/`, gated like the rest of
`/api/v1/_configure`.
