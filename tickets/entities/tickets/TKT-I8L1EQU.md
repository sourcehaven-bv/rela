---
id: TKT-I8L1EQU
type: ticket
title: 'Declarative peer pull: peers.yaml apply side'
kind: enhancement
priority: medium
effort: xl
status: backlog
---

## Description

The declarative apply side of FEAT-CMVZQFI: a `peers.yaml` entry per peer with

- source URL and credential reference;
- scope (types or a view);
- type and field mapping, reusing `ingestspec`;
- the `external_ref` system;
- which fields this side owns;
- `on_gone: detach | delete`.

A scheduler task runs reconciliation, fetches changed rows, and writes through
`entitymanager` as `system:<peer>`. The `sync/<peer>` version tag is the base
for `syncmerge`. A conflict stops that row and is reported. An `_events` signal
can trigger an early run. External calls go through `jobs.Queue`.

Scope: one-way pull. This covers catalogue distribution.

## Acceptance

- End-to-end test: two instances, a catalogue of entities and relations, local additions survive an upstream update.
- `rela analyze` style validation of `peers.yaml` at load.
