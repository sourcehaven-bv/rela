---
id: BUGA-D79ZT54
type: bug-analysis-checklist
title: 'Analysis: remote_mcp config fails open on a null allow and on a decode error'
started: "2026-11-15"
completed: "2026-11-15"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (new TestLoadConfig_RemoteMCP cases fail on the old config.go)
- [x] Minimal reproduction steps documented (`allow: ~`; `other: {a: 1, a: 2}` next to `remote-mcp:`)
- [x] Environment/conditions noted (any backend; config load at rela-server startup)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (check the YAML node in both places)
- [x] Regression test planned (table cases in TestLoadConfig_RemoteMCP)
- [x] Related areas checked for similar issues (deny: null means deny nothing, which is what it says; tools: null means no filter, same as absent)
