---
id: BUGA-5E1L4ZW
type: bug-analysis-checklist
title: 'Analysis: Remotes page asks update right before delete for remote_deleted entities'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (a "delete without update" case in TestRemotes_RemoteDeletedNeedsDelete fails on develop)
- [x] Minimal reproduction steps documented (role with read, delete and remote:sync on ticket but no update; resolve_remote is not offered for a remote_deleted ticket entry)
- [x] Environment/conditions noted (any backend; entity entries only, relation entries already asked delete only)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (branch on remote_deleted before the update check)
- [x] Regression test planned (delete-without-update table case)
- [x] Related areas checked for similar issues (relation path and link/resolve paths reviewed; they ask the right they need)
