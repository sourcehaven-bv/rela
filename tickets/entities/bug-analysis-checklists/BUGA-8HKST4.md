---
id: BUGA-8HKST4
type: bug-analysis-checklist
title: 'Analysis: Remote MCP: faced entities are invisible to every read tool'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (atlas remote MCP: `list_entities type=beleid` and type-scoped search return nothing; code path confirmed)
- [x] Minimal reproduction steps documented
- [x] Environment/conditions noted (faced type with no default-face row; `app.default_world` set; remote MCP; any ACL)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined
- [x] Regression test planned
- [x] Related areas checked for similar issues (web `/_search` has the same defect, filed separately; tracer start node noted as follow-up; stdio MCP shares GatedReads)
