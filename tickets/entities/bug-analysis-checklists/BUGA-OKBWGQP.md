---
id: BUGA-OKBWGQP
type: bug-analysis-checklist
title: 'Analysis: RELA_AFFORDANCE_PROFILE=none/demo disables client field ceilings on REST'
started: "2026-10-31"
completed: "2026-10-31"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (TestResolverFromProfile_Selection none/demo + ceiling cases returned Nop/Demo before the fix)
- [x] Minimal reproduction steps documented (bug body, Reproduction)
- [x] Environment/conditions noted (rela-server with acl.yaml field rules and RELA_AFFORDANCE_PROFILE=none or demo)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (option 1: refuse none/demo when HasAffordanceGrants)
- [x] Regression test planned (table test in dataentry + newServer test in cmd/rela-server)
- [x] Related areas checked for similar issues (rela-desktop wires NopACL so it has no policy; MCP/CLI ignore the profile)
