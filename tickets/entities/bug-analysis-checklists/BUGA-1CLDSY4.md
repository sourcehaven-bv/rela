---
id: BUGA-1CLDSY4
type: bug-analysis-checklist
title: 'Analysis: rela mcp fails to start when flows.yaml names roles'
started: "2026-11-12"
completed: "2026-11-12"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (`rela mcp` on a project made of `internal/flowcfg/testdata`)
- [x] Minimal reproduction steps documented (in BUG-E9A72XB)
- [x] Environment/conditions noted (any host injecting a non-declarative ACL: rela mcp, rela-desktop, read-only rela-server)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] ~~Systemic cause explored (why4-5)~~ (N/A: why3 reaches the missing test; the precedent in newRealtimeConfig shows the pattern was known)

## Fix Planning

- [x] Fix approach determined (flowConfigPolicy loads acl.yaml for the flows check; the engine reuses it)
- [x] Regression test planned (TestFlowEngine_InjectedACLChecksFlowsAgainstACLYAML)
- [x] Related areas checked for similar issues (realtime.yaml already reloads acl.yaml under an injected ACL; query-scope warnings use the loaded policy only, but they warn rather than fail)
