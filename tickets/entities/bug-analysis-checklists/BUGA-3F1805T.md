---
id: BUGA-3F1805T
type: bug-analysis-checklist
title: 'Analysis: The tenant setting-up screen ignores the app language and name'
started: "2026-11-20"
completed: "2026-11-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (the new TestBootShellHandler and bootMeta tests fail on develop: StaticShell serves the built shell with <title>rela</title> and lang=en, and the SPA has no project language before its config loads)
- [x] Minimal reproduction steps documented (tenant mode with -tenant-provision=lazy, a Dutch browser and app.language: en; sign in as a member of a new org: the setting-up screen is Dutch and the tab reads rela)
- [x] Environment/conditions noted (multi-tenant rela-server on postgres; only the tenant-free StaticShell is affected, a tenant's own router loads its config)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (the host reads app.name and app.language from the shared data-entry.yaml once at startup; StaticShell writes them into the title, the html lang and two meta tags; the SPA reads the metas as the project language and the fallback app name)
- [x] Regression test planned (Go tests for ShellBootFrom, bootShell escaping and the handler, shellBoot wiring; vitest for bootMeta, the language and the title)
- [x] Related areas checked for similar issues (the tenant router shell loads the config and is unaffected; the boot screen error states use the same i18n, so they follow too)
