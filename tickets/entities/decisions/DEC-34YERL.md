---
id: DEC-34YERL
type: decision
title: Realtime declarations live in a top-level realtime.yaml with one-way references
context: 'FEAT-GWF8YH phase 3 adds signal types, message channels and entity-change triggers. RES-1UCUBA sub-decision B weighed a realtime: block in schema.yaml, a top-level realtime.yaml, and a split by nature. Jeroen chose realtime.yaml on 2026-10-12.'
consequences: realtime.yaml gets its own loader, validation and hot reload, and is default-locked for Configure until keys are allowlisted. It may reference entity types, views, roles and predicates; schema.yaml never references it, so automations stay unaware of signals and realtime remains removable. Entity-change-to-signal triggers are declared in realtime.yaml. Cross-file references are validated when the file loads.
date: "2026-10-12"
status: accepted
---

## Decision

Declare realtime interaction (signal types, channels, triggers, runtime policy)
in a top-level `realtime.yaml`. References point one way: `realtime.yaml` may
name entity types, views, roles and predicates; `schema.yaml` never names
anything in `realtime.yaml`.

## Alternatives

- `realtime:` block in `schema.yaml`: rejected; mixes durable shape with runtime behaviour and widens the Configure allowlist.
- Split by nature: rejected; the boundary would be argued per key.
