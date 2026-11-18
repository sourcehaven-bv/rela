---
id: BUGA-TCGLVEV
type: bug-analysis-checklist
title: 'Analysis: tenant mode: webhook replay dedup resets when a tenant is evicted'
started: "2026-11-16"
completed: "2026-11-16"
status: done
---

## Reproduction

- [x] Bug reproduced in a test: `TestSeenSet_SurvivesEviction` evicts and reopens the tenant, then delivers the same webhook id again.
- [x] Minimal reproduction steps documented: deliver webhook, evict tenant, reopen, deliver the same JWT again.
- [x] Environment/conditions noted: tenant mode (TKT-D6MF3SW) with eviction; single-project mode is not affected.

## Root Cause

- [x] Immediate cause identified (why1): the seen-set was an in-memory field on the tenant App.
- [x] Contributing factors found (why2-3): eviction drops the App; the seen-set predates tenant mode, when the App lived as long as the process.
- [x] Systemic cause explored (why4-5): per-process state was not reviewed when tenant eviction was introduced.

## Fix Planning

- [x] Fix approach determined: keep the seen-set in the tenant's `state.KV` with a TTL, and answer 503 with Retry-After when the state cannot be read or written.
- [x] Regression test planned: `TestSeenSet_SurvivesEviction`, `TestSeenSet_FailsClosed`, `TestSeenSet_Bounded`.
- [x] Related areas checked for similar issues: other per-tenant background state handled in TKT-N3X235N (due-work sweep, shutdown).
