---
id: BUG-Y1RGTU
type: bug
title: Rename authorizes the zero face but re-keys every face
description: RenameEntity authorizes a faceless subject but renames every face, so a type-wide write grant renames faces it does not cover.
priority: high
effort: s
why1: RenameEntity authorized OpRename against acl.NewFacelessEntitySubject(type, oldID), the zero face, then called store.RenameEntity, which re-keys every face of the family.
why2: The BUG-HC6I2T fix chose a faceless subject on purpose because a rename has no single face; the faceless constructor authorizes the default face, which a faced type does not store and which a bare type-wide grant covers.
why3: Face-qualified write grants arrived after RenameEntity was written for one row per id; the check was not re-derived from the rows the store actually moves.
why4: No rename test paired per-face update grants with a multi-face entity; rename ACL tests used unfaced types or NopACL, so zero-face and every-face authorization looked the same.
why5: Family-wide operations were adapted to faces one call site at a time (DEC-NPZICR); nothing makes authorization cover exactly the rows a write touches, so delete (BUG-1YN750) and rename carried the same defect.
prevention: Rename authorizes every face it moves via the shared authorizeFamily helper, before the Tx and again inside it, and audits and versions each face. Regression tests TestFamilyRename_* in internal/entitymanager/familyrename_acl_test.go run on memstore, fsstore, sqlite and postgres. DEC-NPZICR stages the structural fix.
status: done
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
