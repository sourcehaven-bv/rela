---
id: BUG-Y1RGTU
type: bug
title: Rename authorizes the zero face but re-keys every face
description: RenameEntity authorizes a faceless subject but renames every face, so a type-wide write grant renames faces it does not cover.
priority: high
effort: s
status: backlog
---

## Problem

`Manager.RenameEntity` re-keys the whole family (every face) but authorizes
against the zero face only: `acl.NewFacelessEntitySubject(current.Type, oldID)`
(`internal/entitymanager/manager.go` ~1870). A type-wide write grant (`write:
[policy]`) covers only the zero face today (`acl/worldgrant.go:240-289`), so a
principal holding it, but no `policy@published` grant, can rename the published
face along with the rest of the family.

Verified by reading the code; reported out of scope by the BUG-1YN750 fix
(#1708). Same class as BUG-1YN750.

## Expected

A rename authorizes `update` (or the verb the rename path uses) on every face it
re-keys, and fails closed with no write if any face is denied, as family delete
does since #1708.
