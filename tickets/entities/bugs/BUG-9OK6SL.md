---
id: BUG-9OK6SL
type: bug
title: Check whether a recreated id's history includes its deleted predecessor
description: lineageCTE fences lineage on renames only, so after delete and recreate of the same id the new entity's timeline may include the deleted entity's snapshots, which could belong to an entity hidden from the reader.
priority: medium
status: backlog
---

## Description

`lineageCTE` fences lineage on renames only. After delete and recreate of the
same id, the new entity's timeline may include the deleted entity's snapshots,
which could belong to an entity hidden from the reader. Affects the HTTP history
API and `rela.history` alike.

Found in the TKT-EC7F65 security review. Not confirmed; needs a storetest case
first.
