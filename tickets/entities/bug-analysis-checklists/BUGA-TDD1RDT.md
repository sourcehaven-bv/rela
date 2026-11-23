---
id: BUGA-TDD1RDT
type: bug-analysis-checklist
title: 'Analysis: Tenant pages keep fetching, fixed read-only text and jargon in labels'
started: "2026-11-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (a proxy in front of rela-server that answers the tenant status with suspended and every other API path with 403 tenant_suspended)
- [x] Minimal reproduction steps documented (open the app through the proxy and count the refused paths; open the account menu; open an empty note editor)
- [x] Environment/conditions noted (saas-dev pool behind the edge; any multi-tenant host with a suspended tenant)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (read the tenant status before the router and the mount; record suspended and archived from it; skip the load while a page replaces the app; tab titles on the access screen; account.read_only_text as a shell meta; two label changes)
- [x] Regression test planned (workspaceReadOnly, WorkspaceAccessScreen, ReadOnlyBanner, staticshell and account config tests)
- [x] Related areas checked for similar issues (the root-route guard asks the server, so the router is installed after the status; the event stream is stopped by BUG-JT865RH; a 401 at start still loads under the session dialog)
