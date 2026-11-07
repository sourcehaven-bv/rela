---
id: BUGA-1HL8B1A
type: bug-analysis-checklist
title: 'Analysis: idp-sync.lua keeps roles of a deactivated user and calls a missing Pratique route'
started: "2026-11-07"
completed: "2026-11-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (`TestIdpSyncAction` and `TestIdpSyncAction_ClearsRolesUnlessActive` fail on develop against a stub that serves only Pratique's real operator routes: the action gets 405 on every run)
- [x] Minimal reproduction steps documented (bug body)
- [x] Environment/conditions noted (reference action against Pratique `develop` operator API; routes read from `internal/web/operator_handlers.go`, not executed against a live Pratique)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (fetch the member list, select by user_id, keep roles only for status active, clear otherwise)
- [x] Regression test planned (route-faithful stub; table over inactive, pending, missing status, absent subject, empty list, org 404; error cases keep roles)
- [x] Related areas checked for similar issues (Go webhook receiver and role store are provider-agnostic and unaffected; docs/idp-webhook-provisioning.md described the old 404-only clear and is updated)
