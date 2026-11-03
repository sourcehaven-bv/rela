---
id: TKT-S3YYGM
type: ticket
title: Split liveSub per subscription shape
kind: chore
priority: low
effort: s
status: backlog
---

## Description

liveSub in internal/dataentry/live.go keeps gaining per-shape fields (inbox\,
busy wake\, syncing). Give each shape its own value behind a small interface so
a new live shape does not widen the shared struct (TKT-W5ZSYJ review).
