---
id: BUGA-20GN1Q
type: bug-analysis-checklist
title: 'Analysis: History, restore and history-purge ignore faces'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally
- [x] Minimal reproduction steps documented
- [x] Environment/conditions noted

Reproduced with the e2e spec `faces-history.spec.ts` (fixme) and Go tests on a
faced `ticket` type: `GET /_history/ticket/TKT-1@draft` served the zero-face
lineage, `rela history TKT-1` read an empty lineage, and a restore of a deleted
face minted a new id. Conditions: a type declaring `faces:`, on the postgres or
sqlite backend.

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined
- [x] Regression test planned
- [x] Related areas checked for similar issues

Approach: HTTP history and restore resolve the address through the resolver; the
CLI resolves `ID@face` with `historyAddress` and refuses a bare faced id;
`VersionMeta.Face` is read back (TKT-7R0ABK); a deleted face is recreated with
`ApplyEntity` at its own id and face; purge requests carry the face (entity) and
the tail face (relation). Related areas: relation purge was default-tail only,
fixed here; the SPA hid Restore on non-bare faces, fixed here.
