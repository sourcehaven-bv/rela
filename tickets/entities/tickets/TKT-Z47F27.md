---
id: TKT-Z47F27
type: ticket
title: Whole-entity delete and rename judge hidden faces
kind: enhancement
priority: medium
effort: m
tags: security
status: backlog
---

## Description

`DeleteEntity`, `SoftDeleteEntity` and `RenameEntity` pass the read check when
the caller can read at least one face, and then authorize every stored face,
hidden ones included. A caller who may read and delete only `POL-1@draft` gets a
403 when `POL-1@published` exists and a success when it does not, so the answer
reveals the hidden face. A caller who also holds delete on the hidden face
removes a row they cannot read.

Judge only the faces the caller can read, as `DeleteEntityFace` does: delete the
readable faces, and keep the entity with its hidden faces and their edges when
any remain. Decide the same for rename, where a rename moves every face.

Found in the review of TKT-NXRFDU. The guide documents the current behaviour as
an exception.

## Acceptance

- A whole-entity delete answers the same whether or not a face hidden from the caller exists.
- No delete removes a face the caller cannot read.
