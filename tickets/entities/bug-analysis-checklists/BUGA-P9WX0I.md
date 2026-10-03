---
id: BUGA-P9WX0I
type: bug-analysis-checklist
title: 'Analysis: Family delete authorizes one face and deletes all faces'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (TestFamilyDelete_* fails on memstore and fsstore before the fix)
- [x] Minimal reproduction steps documented (policy type with draft and published faces; ACL `delete: [policy@draft]`; bare-id `DeleteEntity(POL-1)` succeeds and removes both faces)
- [x] Environment/conditions noted (any backend; any faced type with per-face delete grants; reached via CLI delete, MCP delete_entity, Lua delete, HTTP bare-id delete and CalDAV delete)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (enumerate the family, authorize delete per face before the Tx and again for the family re-read inside the Tx, then version-capture and audit each face the store reports deleted)
- [x] Regression test planned (TestFamilyDelete_DeniedUnlessEveryFaceIsDeletable and TestFamilyDelete_EveryFaceCapturedAndAudited over every concBackend)
- [x] Related areas checked for similar issues (RenameEntity authorizes a faceless subject for a family-wide re-key; autocascade host deletes without per-face ACL by design; both noted, out of scope)
