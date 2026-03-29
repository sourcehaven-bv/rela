---
id: IMPL-mv5o
status: done
title: 'Implementation: Define YAML schema types for Query-as-Output-Structure views'
type: implementation-checklist
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: pure data structures, no integration needed)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

1. **Root entry_type/param**: `TestQueryNode_RootFields` - PASS
2. **Nested relations 3+ levels**: `TestQueryNode_NestedRelations` - PASS (4 levels)
3. **Traversal options**: `TestQueryNode_ViaOutgoing`, `TestQueryNode_ViaIncoming`, `TestQueryNode_TypesFilterSingle/Multiple`, `TestQueryNode_Recursive` - PASS
4. **Require with JSONPath**: `TestQueryNode_RequireWithJSONPath`, `TestQueryNode_RequireMultipleRelations` - PASS
5. **Output controls**: `TestQueryNode_OnlyProperties`, `TestQueryNode_ContentFalse/Default`, `TestQueryNode_PropsFalse/Default` - PASS
6. **V1 compatibility**: `TestIsV2Format` confirms v1 detection works - PASS
7. **Complex example**: `TestQueryNode_ComplexExample` tests realistic use case - PASS

All 26 tests pass:
```
go test ./internal/views/... -v
ok  	github.com/Sourcehaven-BV/rela/internal/views	1.358s
```

## Quality

- [x] Code follows project patterns (check similar code)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

**Files created:**
- `internal/views/types_v2.go` - New types (QueryNode, ViewDefV2, FileV2)
- `internal/views/types_v2_test.go` - Comprehensive tests

**Files modified:**
- `internal/views/loader.go` - Added LoadV2, ParseV2, IsV2Format functions
