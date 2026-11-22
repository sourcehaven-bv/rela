---
id: BUGA-3MROUDT
type: bug-analysis-checklist
title: 'Analysis: Suspended, archived and ended-session states'
started: "2026-11-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (saas-dev: suspend and archive a tenant, let the session expire, make a tenant read-only)
- [x] Minimal reproduction steps documented (suspend a tenant and open the app: toast "unknown tenant"; let the edge session expire: network error and Retry)
- [x] Environment/conditions noted (multi-tenant postgres server behind an edge that signs users in)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (a code per tenant state; one SPA module that records whole-workspace refusals; a page per refusal and a dialog for an ended session; boot meta from data-entry.yaml; read-only banner and gates)
- [x] Regression test planned (TestHost_FailsClosed, TestProvision_StatusRoute, TestClusterResolver_Archived, workspaceAccess.test.ts, WorkspaceAccessScreen.test.ts, ReadOnlyBanner.test.ts)
- [x] Related areas checked for similar issues (the SSE stream also hid an ended session; it now asks too)
