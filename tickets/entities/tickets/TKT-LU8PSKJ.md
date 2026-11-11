---
id: TKT-LU8PSKJ
type: ticket
title: 'Flows: fs and memory backends do not order run writes against field writes (accepted risk)'
kind: chore
priority: low
status: backlog
---

## Description

Accepted risk, recorded so it stays visible. Found during TKT-XI7IDBS.

A field write checks a flow's `outside_edits` ceiling, then writes. On the
postgres and sqlite backends, the write asks the ceiling again inside its store
transaction, and flow run writes take the same write lock. A run that starts
between the check and the write therefore refuses the write.

On the fs and memory backends, the run store is a KV store that shares no
transaction with the entity store, and run writes do not take the entity store's
write lock. The write still asks the ceiling again inside its transaction, which
narrows the window, but nothing orders a run write against that check: a run can
commit just after it, in the same process or in another one (for example the
`rela` CLI next to `rela-server` on the same project). That one write then lands
on a property the run's ceiling denies. Later writes see the run and are
refused.

Conflict resolution in the data-entry app writes the resolved file directly (fs
only) and is covered by this same residual.

## Decision

The user decided that fs gets no lock for this (TKT-XI7IDBS review): no file
lock, and no lock that applies across backends. The fs tier is the single-user,
local tier. A deployment that needs the guarantee uses sqlite or postgres.

## When to revisit

- fs is used for multi-process or multi-user deployments in practice.
- A flow's `outside_edits` is used to protect data whose integrity matters
more than the fs tier promises.

## References

- docs/flows.md, section on edits outside a flow.
- PLAN-9PX91CT threat model.
