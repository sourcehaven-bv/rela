---
id: BUGA-Y21W37
type: bug-analysis-checklist
title: 'Analysis: Tracer and analyze skip faced types'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (a faced-only type was absent from orphans, duplicates, gaps and trace on the base branch)
- [x] Minimal reproduction steps documented (seed a type with `faces:` and no default row, run `rela analyze orphans` or `rela trace from`)
- [x] Environment/conditions noted (any backend; only types that declare faces)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (stage2-design section 5.3: family nodes in the tracer, per-check coverage, gate before fold under ACL)
- [x] Regression test planned (faced fixtures in tracer, visibility, analysis, CLI, MCP and data-entry analyze, including a principal who cannot read one face)
- [x] Related areas checked for similar issues (cardinality and properties already scanned every face; trace edges hung from a hidden face were also shown and are now withheld)
