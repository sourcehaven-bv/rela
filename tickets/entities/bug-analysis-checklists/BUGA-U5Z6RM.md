---
id: BUGA-U5Z6RM
type: bug-analysis-checklist
title: 'Analysis: rename.go upsertEntity/upsertRelation retain pre-BUG-ZWTDH9 create-then-Update-on-ErrConflict fallback (lost-update/clobber)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] ~~Bug reproduced locally~~ (N/A: race requires a postgres multi-writer harness; established by code analysis. The Update-fallback is reachable only on a Create ErrConflict, which validatePreconditions makes impossible except under a concurrent create — so the fallback firing IS the bug. Confirmed statically, not run live.)
- [x] Minimal reproduction steps documented (two writers rename-to / create the same newID concurrently on pgstore: writer A's validatePreconditions sees newID free, writer B creates it, A's CreateEntity returns ErrConflict, A falls through to UpdateEntity and clobbers B's row)
- [x] Environment/conditions noted (postgres build only; multi-writer cross-process via LISTEN/NOTIFY. fsstore/memstore are single-writer so the window is not reachable there)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (delete upsertEntity/upsertRelation; strict CreateEntity/CreateRelation; map store.ErrConflict -> ErrEntityAlreadyExists so a lost race fails loudly. Verified safe: newID is required free and every renamed relation key is fresh, so Create can only conflict on a race)
- [x] Regression test planned (rename-no-overwrite-test: conflictOnCreateStore counts Update* calls, asserts zero + ErrEntityAlreadyExists; mirrors create-no-overwrite-test / BUG-R2PV8G)
- [x] Related areas checked for similar issues (grepped all ErrConflict handlers: rename.go is the SOLE Create-then-Update-on-conflict fallback left. entitymanager/apply.go Apply* is the sanctioned resolve-by-intent path and stays. P5 follow-up: arch-lint rule to forbid the idiom recurring)
