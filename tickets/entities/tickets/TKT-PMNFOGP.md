---
id: TKT-PMNFOGP
type: ticket
title: Limit replay load from forged postgres channel hints
kind: enhancement
priority: low
effort: s
status: backlog
---

## Description

On PostgreSQL, channel hints (TKT-YK30D1) arrive as NOTIFY payloads on
`rela_channels`. NOTIFY needs no privilege, so any session on the database can
send a payload that names a schema. The receiving processes accept it as a real
hint.

The follower limits gaps and resync hints to one full resync per 5 seconds.
Message, moved, addressed and read hints are applied directly. About 16 to 20
valid payloads per live-hub tick, each with 64 distinct keys, push a subscriber
past `MaxSubscriberChanges` (1024). That sets a full resync outside the
follower's limit, every tick, on every process of the schema.

Each replay is still an ACL-checked read as the subscriber, so the effect is CPU
and database load, not disclosure. On the file backend only someone who can
already write the channel store can do this. TKT-YK30D1 documents the behaviour
in `docs/postgres-backend.md`.

## Options

1. Count distinct received keys per window in the follower. Turn an excess
into `requestResync`, which is already rate-limited.
2. Authenticate hints: an HMAC per payload with a key kept in a table in the
tenant's own schema; drop payloads that fail the check.
3. Solve together with option 2 of TKT-147C19D (hint rows in a schema
table, NOTIFY carries only a row id), which needs write access to the schema to
forge.

Found by the security review of TKT-YK30D1.
