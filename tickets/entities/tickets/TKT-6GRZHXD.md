---
id: TKT-6GRZHXD
type: ticket
title: Peer pull hardening after review
kind: enhancement
priority: low
effort: m
status: backlog
---

## Description

Follow-ups from the code review of TKT-I8L1EQU (the `peers.yaml` pull):

- Compare-and-delete: `entitymanager.DeleteEntity` has no expected-version form, so an edit saved between the "edited locally" check and the delete is lost (RR-3QG82IE).
- Batch the external-ref lookup for new rows per page instead of one graph query per row (RR-AV17OW3).
- Shard or cap the per-peer ledger, which is one `state.KV` document rewritten per run (RR-GE6WC2T).
- Refuse, at load, a mapping that writes the `acl.yaml` `principal_property` of a type (RR-TOJ0G92).

## Acceptance

- A pull's delete applies only to the version it checked.
- An initial pull of N new rows runs a bounded number of ref queries per page.
- A ledger of `max_rows` rows does not need one document of that size.
- A mapping that owns the principal property fails `rela validate`.
