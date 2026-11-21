---
id: BUGA-LEH9PAC
type: bug-analysis-checklist
title: 'Analysis: Read-only workspace wording and create form'
started: "2026-11-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (a vitest form test with the dry-run refused by a 403 tenant_read_only rendered no fields)
- [x] Minimal reproduction steps documented (make a tenant read-only, open a create form: no fields; try to save anything: toast "The tenant is read-only")
- [x] Environment/conditions noted (multi-tenant postgres server with a read-only or provisioning tenant)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (workspace wording on the server and in both languages; the failed dry-run leaves the form unfiltered; a client-wide read-only flag drives a notice and hides Create)
- [x] Regression test planned (DynamicForm.readonly.test.ts, workspaceReadOnly.test.ts, TestReadOnly)
- [x] Related areas checked for similar issues (the same catch hid fields on any dry-run failure such as a 500 or a network error; fixed by the same change)
