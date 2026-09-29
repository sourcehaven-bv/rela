---
id: BUGA-JX9DYC
type: bug-analysis-checklist
title: 'Analysis: Deleting a face with a content-scoped edge is refused: relation check reads the zero face'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (TestDeleteEntityFace_ContentEdgeAuthorizedByItsTail fails on memstore and fsstore with `no role grants delete on relations from type ""`)
- [x] Minimal reproduction steps documented (policy with draft and published faces; `implements` scope content; POL-1@draft implements CTL-1; ACL `delete: [policy@draft]`; DeleteEntityFace(POL-1, draft) is refused)
- [x] Environment/conditions noted (any backend; any faced source with a content-scoped edge; face delete, family delete and cascade delete of a faceless target all go through authorizeCascadeRelations)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (resolve the source type at the edge's tail face through the Tx view, fall back to any face of the family, keep "" for an unresolvable source and surface store errors)
- [x] Regression test planned (faceedge_delete_acl_test.go over every concBackend: allowed, denied with nothing written, family and faceless-target cascades)
- [x] Related areas checked for similar issues (CreateRelation, UpdateRelation and DeleteRelationState already use anyFaceOf; the remaining zero-face read in manager.go is in RenameEntity, handled by BUG-Y1RGTU)
