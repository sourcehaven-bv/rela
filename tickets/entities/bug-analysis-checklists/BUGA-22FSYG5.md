---
id: BUGA-22FSYG5
type: bug-analysis-checklist
title: 'Analysis: The tenant setting-up screen still ignores the app language'
started: "2026-11-21"
completed: "2026-11-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (curl of the pool shell on 127.0.0.1:8210 shows lang="en", the app name and both rela-* metas, so the server side works; useShellAppConfig.test.ts with the old watch fails: a Dutch browser wins over the boot language)
- [x] Minimal reproduction steps documented (saas-dev pool: data-entry.yaml app.language en, a Dutch browser, sign in as a member of a new org; the setting-up screen is Dutch)
- [x] Environment/conditions noted (multi-tenant rela-server with -tenant-provision lazy at develop f71d489e0; the saas-dev process-compose needs no extra flag, the server reads projects/pool/data-entry.yaml)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (a composable that keeps the shell values until schemaStore.loaded, then applies the config as before)
- [x] Regression test planned (mount the composable over a reactive store before and after load)
- [x] Related areas checked for similar issues (other schemaStore.app readers run after load inside routed views; App.vue was the only one running before it)
