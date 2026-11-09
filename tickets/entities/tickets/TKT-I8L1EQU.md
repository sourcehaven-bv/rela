---
id: TKT-I8L1EQU
type: ticket
title: 'Declarative remote pull: remotes.yaml apply side'
kind: enhancement
priority: medium
effort: xl
started: "2026-11-08"
completed: "2026-11-09"
status: done
---

## Description

The declarative apply side of FEAT-CMVZQFI: a `remotes.yaml` entry per remote
with

- the remote URL and a credential reference;
- scope (types or a view);
- type and field mapping, reusing `ingestspec`;
- the `external_ref` system;
- which fields this side owns;
- `on_gone: detach | delete`.

A pull runs reconciliation, fetches changed rows, and writes through
`entitymanager`. The `sync/<remote>` version tag is the base for `syncmerge`. A
conflict stops that row and is reported. An `_events` signal can trigger an
early run. External calls go through `jobs.Queue`.

Scope: one-way pull. This covers catalogue distribution.

## Identity and credential (DEC-QKLMMD4, replaces DEC-81161C3)

rela issues no credentials of its own. On the remote, this instance is an
ordinary principal: it authenticates at the remote's identity edge (a
machine-client token from Pratique or another OIDC proxy, or the proxy's
principal header), and the remote's `acl.yaml` decides what it reads.

On this side the pull runs as the local principal that starts it: the CLI user,
or the identity the scheduler binds for the task. It is not `system:<remote>`.
Its writes are gated and audited like any write by that principal.

The credential reference in `remotes.yaml` names where the token comes from: an
environment variable, a file, or a global secret under `remote_credential.` (the
same sources as `rela reconcile diff --token-env`, `--token-file` and
`--token-secret`). The token never appears in `remotes.yaml`, and no script can
read a `remote_credential.` secret. Config names say "remote", not "peer".

## Threat model note: mass delete through `on_gone: delete` (issue #2119)

"Gone" means the remote no longer lists the row to this instance's identity. A
deleted row and a hidden row look the same, as the ACL requires. So a revoked
role, a narrowed `acl.yaml`, a revoked or expired token at the remote, or a
misconfigured scope makes every row in scope look gone at once. With `on_gone:
delete` this side would then delete them all, without a threshold or a
confirmation.

The design must address this before `delete` ships, for example:

- refuse to apply a run whose gone set exceeds a threshold (a count or a share
of the local rows from this remote), and report it instead;
- treat a 401 or 403 from the remote, or an empty scope, as a failed run,
never as "everything is gone";
- make `detach` the default and require `delete` to be explicit;
- record every delete in the audit log under the running principal with the
run id.

## Acceptance

- End-to-end test: two instances, a catalogue of entities and relations, local additions survive an upstream update.
- `rela analyze` style validation of `remotes.yaml` at load.
- A run that would delete more than the threshold applies nothing and reports why.
