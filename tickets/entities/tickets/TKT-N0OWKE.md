---
id: TKT-N0OWKE
type: ticket
title: 'Harden pgstore version-write atomicity: capture content inside the entity write tx'
kind: enhancement
priority: low
effort: m
status: backlog
---

Follow-up to TKT-9INY0Y / RES-4ILUJZ (option B3).

## Context

pgstore content versioning v1 (TKT-9INY0Y) writes the `entity_versions` row at
the **entitymanager boundary, post-commit and best-effort** (option B1) —
consistent with how audit already works. RES-4ILUJZ assessed the non-atomicity
as low-severity: a crash in the ~microsecond window between the store's
`tx.Commit()` and the version-write round-trip loses **one** historical snapshot
(not user content, not corruption, not mis-attribution of surviving rows), and
full snapshots make the gap self-healing on the next write.

## What this ticket does

Harden atomicity to **B3 (hybrid)** *if a real deployment shows the B1 gap
biting* (e.g. the S1 auditor scenario demands strict revision continuity):
pgstore captures the **content snapshot inside the same transaction** as the
entity `UPDATE` (like the tombstone write already does), while **attribution
stays at the entitymanager boundary** (the store still never learns the
Principal — a version row can briefly exist as "content preserved, actor
pending", joined post-commit).

## Precondition

The v1 version-write hook should be shaped so this move is mechanical —
RES-4ILUJZ decision #1 calls for exactly that. Don't start this until B1 is
shown insufficient; it's a hardening, not a v1 requirement.
