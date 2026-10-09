---
id: TKT-2Z2IA6
type: ticket
title: Copy option to keep named fields on the target face
kind: enhancement
priority: medium
status: backlog
description: 'A copy with fields: all replaces the target face and drops fields that only the target holds. A file added to an adopted face is lost on the next adoption. Add a copy option that keeps named target fields (GitHub #1760 part 4).'
---

## Description

A copy with `fields: all` replaces the target face and drops fields that exist
only on the target (`docs/content-states.md`, copies). In the #1760 use case, a
signed PDF attached to the `vastgesteld` face after adoption disappears when the
document is adopted again.

## Proposal

A copy option, for example `keep: [ondertekend]`, that preserves the named
fields on the target. It must define what happens to the target file bytes when
the source has a value for the same field, and how the attachment reference
counting treats kept files.
