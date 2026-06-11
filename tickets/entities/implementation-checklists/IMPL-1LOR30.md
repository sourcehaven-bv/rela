---
id: IMPL-1LOR30
type: implementation-checklist
title: 'Implementation: Hot-path benchmarks: dry-run validation, affordances, search, write validation'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written — TestValidateCreate_AllocCeiling (enforced contract from the review's leverage finding); benchmarks themselves are the deliverable
- [x] Integration — benchmarks drive real service paths (Declarative member-of walk, bleve, fresh-LState Lua)
- [x] Happy path implemented (4 benchmark files + just bench)
- [x] Edge cases handled (hits==0 fixture guard in search; AllocsPerRun serial constraint documented)
- [x] Error handling in place (b.Fatal on all setup errors)

## Test Quality

- [x] Fixtures reuse package conventions (UC10 shape, testMetamodelYAML, newMockWorkspace)
- [x] Reviewer verified non-vacuity of all three suspicious paths: role walk resolves through member-of (not the no-roles early exit), search query matches 250 rows, ValidateCreate validates rather than failing fast and uses SkipIDGeneration (no ID scan)
- [x] Setup outside b.Loop everywhere; b.ReportAllocs on all five; dead-code elimination not a hazard (interface dispatch + side effects)

## Manual Verification

- [x] All benchmarks smoke-run; numbers: ValidateCreate 1.4µs/7allocs, Verdicts 8.6µs/107allocs, Search 2.5ms, Check_WhenThen 16µs vs Check_Lua 2.5ms (~150× — the per-write fresh-LState cost, now a number)
- [x] Reviewer reproduced the alloc counts exactly at -benchtime=20x
- [x] just ci green; touched packages green under -race -count=2 -shuffle=on

## Quality

- [x] Follows existing benchmark conventions (pgstore/lua precedents)
- [x] No security issues; no silent failures; no debug code
