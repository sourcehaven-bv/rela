---
id: TKT-D055SV
type: ticket
title: Index version_tags by name
kind: chore
priority: low
status: backlog
---

## Description

`rela.tagged_entities` (TKT-7LRUPB) filters `version_tags` by `name`. The
primary key is `(vseq, name)`, so the lookup scans the tag table. Add an index
on `version_tags(name)`: a PostgreSQL migration and a SQLite schema version
bump. Measure first; the table only grows by explicit tagging.

Deferred from RR-MYCY4Q.
