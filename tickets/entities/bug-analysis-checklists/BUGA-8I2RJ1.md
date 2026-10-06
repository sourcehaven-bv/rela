---
id: BUGA-8I2RJ1
type: bug-analysis-checklist
title: 'Analysis: Create sets a hardcoded status when the schema declares no default'
started: "2026-10-06"
completed: "2026-10-06"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally
- [x] Minimal reproduction steps documented
- [x] Environment/conditions noted

A schema with a `note` type that has no `status` property. `rela create note -P
title=hello` writes `status: draft`. A `task` whose status type is `values:
[new, done]` with no `default:` gets `status: new`. Built from `origin/develop`,
filesystem store. The same happens through the HTTP API, MCP `create_entity`,
Lua `rela.create_entity` and `rela import`; new Go tests on each path fail
before the fix.

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

See why1 to why5 on the bug.

## Fix Planning

- [x] Fix approach determined
- [x] Regression test planned
- [x] Related areas checked for similar issues

`GetDefaultStatus` returns only declared values: property `default:`, else the
named type's `initial:`, else its `default:`; otherwise "". Callers leave
`status` unset on "". The same rule, as `metamodel.DeclaredDefault`, now drives
`rela template init`, which used to write the first enum value, `false` and `0`
into generated templates. Tests per create path: metamodel unit, entitymanager,
importer (dry run and write), MCP and Lua via MCP, HTTP create, template
generation, and an e2e form create.

Related areas checked and left: display sort order in `internal/output`, the
legacy `status` value list in `internal/filter` (read side only), `perfseed`
(synthetic benchmark data), `DefaultMetamodelYAML` (scaffolded config with an
explicit `default: draft`).
