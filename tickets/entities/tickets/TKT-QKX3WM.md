---
id: TKT-QKX3WM
type: ticket
title: Version tags on relations
kind: enhancement
priority: low
status: backlog
---

## Description

Version tags name entity versions only. A baseline of a set of entities
(TKT-7LRUPB, `rela.tagged_entities`) therefore cannot freeze the relations
between them: a compare script sees the tagged entity content but the current
relations.

Extend version tags to relation versions (`relation_versions`, TKT-92JL8P) on
SQLite and PostgreSQL: tag, untag, read by tag and list by tag, gated on both
endpoints like relation history reads.

Out of scope of TKT-7LRUPB (brief G5).
