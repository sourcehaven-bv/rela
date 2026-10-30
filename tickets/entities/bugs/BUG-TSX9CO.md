---
id: BUG-TSX9CO
type: bug
title: fsstore index can go stale and hide an existing entity
description: fsstore-index.json listed every RR of a create batch except RR-LK1YGE, so rela answered entity not found for a valid file until the index was rebuilt.
priority: medium
status: backlog
---

## Description

On 2026-10-30 RR-LK1YGE (created by `rela create` in a batch of 21) existed on
disk with valid frontmatter, but every command answered "entity not found".
`tickets/.rela/fsstore-index.json` did not list it while its sibling RRs from
the same batch were listed; touching the file did not help. Moving the index
aside made rela rebuild it and the entity appeared.

The index trusts `entities_dir_mtime`, so a save that raced a create (or an
mtime with coarse resolution) can record the new directory mtime without the new
entity, and the file is then never rescanned. The machine was under heavy load
(load average about 100) at the time. The stale index is kept at
/tmp/fsstore-index.stale.json for inspection.
