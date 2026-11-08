---
id: TKT-I8L1EQU
type: ticket
title: 'Declarative peer pull: peers.yaml apply side'
kind: enhancement
priority: medium
effort: xl
started: "2026-11-08"
completed: "2026-11-08"
status: done
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

## Credential (from TKT-H91N91L, DEC-81161C3)

The source authenticates the peer with a peer key (`Authorization: Bearer
relapeer_...`). The source stamps `peer:<name>` and gates its reads with the
source ACL. The credential reference in `peers.yaml` names a secret in this
instance's `.rela/secrets.yaml`; the key never appears in `peers.yaml`. The
`system:<peer>` principal on this side is the local writer only, for audit; it
is never a request identity.

## Threat model note: mass delete through `on_gone: delete` (issue #2119)

"Gone" means the source no longer lists the row to this peer. A deleted row and
a hidden row look the same, as the ACL requires. So a revoked role, a narrowed
`acl.yaml`, a removed peer key digest or a misconfigured scope at the source
makes every row in scope look gone at once. With `on_gone: delete` the target
would then delete them all, without a threshold or a confirmation.

The design must address this before `delete` ships, for example:

- refuse to apply a run whose gone set exceeds a threshold (a count or a share
of the local rows from this peer), and report it instead;
- treat a 401 or 403 from the source, or an empty scope, as a failed run,
never as "everything is gone";
- make `detach` the default and require `delete` to be explicit;
- record every delete in the audit log as `system:<peer>` with the run id.

## Acceptance

- End-to-end test: two instances, a catalogue of entities and relations, local additions survive an upstream update.
- `rela analyze` style validation of `peers.yaml` at load.
- A run that would delete more than the threshold applies nothing and reports why.
