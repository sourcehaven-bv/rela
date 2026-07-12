---
id: TKT-VFJKMB
type: ticket
title: Relation content versioning in pgstore (first-class, triple-keyed history)
kind: enhancement
priority: low
effort: l
status: backlog
---

Follow-up to TKT-9INY0Y / RES-4ILUJZ (option D, deferred).

## Context

pgstore content versioning v1 (TKT-9INY0Y) is **entities-only**. RES-4ILUJZ
rejected embedding relations into an entity's snapshot (option D2) because a
relation is **shared between two entities** (an independent row with its own PK,
`from_id/rel_type/to_id`): embedding would silently mutate the un-written
entity's history or duplicate the relation into both. First-class relation
versioning was chosen as the correct future shape.

## What this ticket does

Add **first-class relation version history** in pgstore: a `relation_versions`
table keyed by the relation triple (mirroring `entity_versions`), capturing
`{properties, content}` per write with the same attribution + `schema_hash`
model as entity versions.

## Why (scenario coverage it completes)

RES-4ILUJZ's scenarios are only partially served by entities-only:
- **S1 (auditor)** / **S3 (reviewer)** currently see entity content+property history but **not how an entity's relation set changed over time**.
- **S2 (restore)** currently recovers content/props but **not the as-of relation set**.

This ticket closes those gaps. Decide during its own planning whether
restore-of-relation-set (reconstructing an entity's relations as-of a version)
is in scope or a further follow-up.
