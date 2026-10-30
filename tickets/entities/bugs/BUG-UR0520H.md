---
id: BUG-UR0520H
type: bug
title: rela script timeout is too short for a seed on the fs backend
description: A full demo seed on fs still needs more than the fixed 30 s script timeout because each write waits for fsync.
priority: low
status: backlog
---

## Description

After BUG-TT73D0P, a full seed of the GRC demo (341 entities, 508 relations) on
the fs backend still takes 41-52 s on a loaded machine. Most of that is fsync
waits, not CPU (about 6 s CPU). `rela script` has a fixed 30 s timeout
(`lua.DefaultTimeout`) and no flag to change it, so the seed cannot finish.

## Options

1. **A `--timeout` flag on `rela script`.** Seeding and migrations are
legitimate long runs. Smallest change; it does not make fs writes faster.
2. **Set scorch `unsafe_batch` on the persistent bleve index.** Measured:
wall time 52 s to 30 s. TKT-Z678TN rejected it because `Close` can drop recent
writes. Since then the startup check only trusts an index whose backfill
watermark covers the store's newest mtime, and any write after a backfill moves
that mtime past the watermark, so a lost write is reindexed on the next start.
Remaining risk: on a filesystem with 1 s mtime resolution, a write in the same
second as the watermark sample does not move the mtime past it.
3. Batch the markdown fsyncs inside one script run (larger change to the
fsstore write path).
