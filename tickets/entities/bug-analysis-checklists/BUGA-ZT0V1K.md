---
id: BUGA-ZT0V1K
type: bug-analysis-checklist
title: 'Analysis: Client field ceilings are ignored when no role declares field grants'
started: "2026-10-22"
completed: "2026-10-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (TestCeilingOnlyPolicy_RedactsField and TestCeilingE2E_FieldCeilingWithoutRoleFieldGrants fail on develop: the app client reads and searches the redacted field)
- [x] Minimal reproduction steps documented (acl.yaml with one role `read: ["*"]`, no field blocks, and `client_baselines: {apps: {applies_to: [app], redact: {person: [salary]}}}`; read PERS-1 as a verified app principal)
- [x] Environment/conditions noted (all backends; any policy whose only field restriction is a client ceiling)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (count client baseline `visible:`/`redact:` in `Policy.HasAffordanceGrants`, the single predicate both entry points use)
- [x] Regression test planned (acl unit table, appbuild read+search, dataentry HTTP GET/list/search)
- [x] Related areas checked for similar issues (only two callers of HasAffordanceGrants, both covered; scope grants only re-open, so they need no resolver on their own; the field write gate is unaffected because ceilings only narrow `visible`)
