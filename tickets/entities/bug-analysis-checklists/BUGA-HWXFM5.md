---
id: BUGA-HWXFM5
type: bug-analysis-checklist
title: 'Analysis: sqlite history attributes create/update versions to version-sweep instead of the editor'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally
- [x] Minimal reproduction steps documented
- [x] Environment/conditions noted

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined
- [x] Regression test planned
- [x] Related areas checked for similar issues

## Notes

Reproduced with a sqlite build driving `rela mcp` over stdio. Postgres with the
same steps attributes correctly. Rename and delete were already correct on
sqlite (synchronous capture).

Fix: add last_edited_by_user/_tool to sqlite entities and relations (schema v7
with an idempotent ladder rung), stamp store.AttributionFrom(ctx) on create and
update, and copy the columns onto swept versions.

Regression test: storetest.RunSweepAttributionTests, required for every backend
that declares Capabilities.Versioning.

Related: sqlite live rows also lack pgstore's origin_* columns, so swept
versions of copied entities lose their copy provenance. Filed separately.
