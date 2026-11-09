---
id: FEAT-CMVZQFI
type: feature
title: Rela-to-rela sync by set reconciliation
summary: 'Sync parts of the graph between two rela instances: stateless range-based set reconciliation over the rows a remote may read, applied by a declarative remote spec.'
description: Sync parts of the graph between two rela instances. The client finds what was added, changed or is gone by range-based set reconciliation over the rows it may read; the server keeps no state per client. A declarative remotes.yaml maps and applies the result through entitymanager as the local principal that runs the pull, with a ledger per remote as the merge base. Builds on FEAT-XYQMUB.
priority: medium
status: proposed
---

## Summary

Sync parts of the graph between two rela instances. The client finds out what
changed by **set reconciliation** over the rows it may read: it compares
fingerprints of id ranges with the server until it knows which rows were added,
changed or are gone. The server keeps no state per client. A declarative remote
spec maps and applies the result through `entitymanager`.

## Use cases

Most real cases avoid conflicts by giving each type or field one owner.

- **Publish and subscribe with local additions**: a central catalogue of ISO
27001, NEN 7510, BIO or AVG requirements distributed to customer instances;
sector threat information; supplier assurance.
- **Two-way with split ownership**: processor and controller (processing
register, data breach notices), auditor findings, consultant and client, joint
projects, upstream and downstream issue tracking.
- **Read-only aggregation**: group consolidation, a consultant portfolio.
- **Full replica** (offline laptop): out of scope. It needs automations to run
on one side only and full fidelity, which is a different product.

## Design

- **Element**: the pair (address, view hash). The address is `type/id@face`
for an entity and `from--type--to` for a relation. A relation counts only when
both endpoints are visible and in scope.
- **View hash**: the canonical hash of the row as the reader sees it, using the
same rule as the ETag (BUG-J01V0Q), so a hidden field never shows up in it.
- **Reconciliation**: range-based, as in Negentropy. The fingerprint of a range
is an additive sum of element fingerprints, computed in SQL with the ACL row
predicate. A range that matches stops; a range that differs is split or listed.
No change costs one round trip.
- **Gone**: an id the client holds that the server no longer lists. Deleted and
hidden look the same, as the ACL requires. The remote spec decides between
`detach` and `delete`.
- **No change feed is needed for sync.** `_events` only tells a client to
reconcile early.
- **Apply**: a declarative `remotes.yaml` with source, credential source,
scope, `ingestspec` mapping, `external_ref` system, field ownership and
`on_gone`. Writes go through `entitymanager` as the local principal that runs
the pull (the CLI user, or the scheduler's identity); the ledger in `state.KV`
is the base for `syncmerge`.
- **Identity** (DEC-QKLMMD4): rela issues no credentials of its own. On the
source, the puller is an ordinary principal authenticated at the identity edge;
the credential comes from a `REMOTE_CREDENTIAL_` variable, a
`remote_credential.` file or a `remote_credential.` secret.
- **Two-way** is two one-way pulls, each side pulling the fields the other owns.

This builds on FEAT-XYQMUB (external refs, field ownership, `syncmerge`, system
principals) and replaces the hash-and-cursor design of the removed FEAT-NJ9FEN.
See the linked research for the options that were compared.

## Steps

1. Define the element and the view hash.
2. Compute `content_hash` at write time and add a fingerprint column.
3. Store capability for range fingerprints, with conformance tests.
4. View hash for types with redacted fields.
5. `POST /api/v1/_reconcile`.
6. Go client and `rela reconcile` diff command.
7. Remote identity: an ordinary principal at the source's identity edge.
8. Declarative apply side (`remotes.yaml`).
9. Two-way through mutual pulls.
10. Status per remote, conflict list and guides.

Steps 1 to 6 already give a useful diff between two instances.

## Open questions

- **Who is the first user?** Sync was removed once because nobody used it
(ruling D9). Catalogue distribution is the proposed first case.
- How often to run a full reconciliation, and how `_events` triggers an early
one.
- Whether attachments and comments travel, and how.
