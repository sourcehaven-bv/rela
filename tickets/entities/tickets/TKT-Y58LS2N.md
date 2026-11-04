---
id: TKT-Y58LS2N
type: ticket
title: Mint batch ids once per type
kind: refactor
priority: low
effort: s
status: backlog
---

## Description

`entitymanager.CreateBatch` generates each id through `createCore`, which scans
every stored id of the type (`collectAllIDs`) once per generated id. A batch of
100 entities of one type scans 100 times while it holds the deployment-wide
write lock. Other writers wait for that.

Mint the ids of one type once per batch: scan once, then generate every id of
that type against the same set plus the ids minted so far.

## Acceptance criteria

- A batch of N entities of one type scans the stored ids of that type once.
- A budget test pins the number of scans.
- Ids stay unique within the batch and against concurrent creates.

Deferred from RR-2EGZ18C (TKT-JRP3ZRX).
