---
id: PLAN-jc0p
status: done
title: 'Planning: Define YAML schema types for Query-as-Output-Structure views'
type: planning-checklist
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN SCOPE:
- Define Go types for query-as-output-structure format
- Support traversal: `via`, `via_incoming`, `types` (list), `recursive`
- Support filtering: `where`, `require` (JSONPath strings stored, not evaluated)
- Support output control: `only`, `content`, `props`
- Explicit `relations:` block for children (no custom unmarshal needed)
- Coexist with existing `ViewDef` type

OUT OF SCOPE:
- Engine implementation (evaluating the query)
- JSONPath parsing/evaluation
- CLI/MCP changes
- Migration tooling
- Validation against metamodel

**Acceptance Criteria:**

1. Can unmarshal a view with entry_type and param at root level
2. Can unmarshal nested relations (3+ levels deep)
3. Can unmarshal all traversal options (`via`, `via_incoming`, `types`, `recursive`)
4. Can unmarshal `require` with JSONPath strings
5. Can unmarshal output controls (`only`, `content`, `props`)
6. Existing views.yaml files continue to work (no breaking changes)

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

With explicit `relations:` block, standard Go YAML unmarshaling handles recursion automatically via `map[string]*QueryNode`. No custom unmarshal needed.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

```go
// QueryNode represents a node in the query tree
type QueryNode struct {
    // Entry point (root only)
    EntryType string `yaml:"entry_type,omitempty"`
    Param     string `yaml:"param,omitempty"`
    
    // Traversal
    Via         string   `yaml:"via,omitempty"`
    ViaIncoming string   `yaml:"via_incoming,omitempty"`
    Types       []string `yaml:"types,omitempty"`
    Recursive   int      `yaml:"recursive,omitempty"`
    
    // Filtering
    Where   string            `yaml:"where,omitempty"`
    Require map[string]string `yaml:"require,omitempty"`
    
    // Output control
    Only    []string `yaml:"only,omitempty"`
    Content *bool    `yaml:"content,omitempty"`
    Props   *bool    `yaml:"props,omitempty"`
    
    // Children
    Relations map[string]*QueryNode `yaml:"relations,omitempty"`
}

// ViewDefV2 is the new view format (root IS a QueryNode)
type ViewDefV2 struct {
    QueryNode `yaml:",inline"`
    Description string `yaml:"description,omitempty"`
}
```

**Example YAML:**

```yaml
views:
  document_publish:
    description: "Complete context for document publishing"
    entry_type: document
    param: doc_id
    
    relations:
      bouwbloks:
        via: describesBouwblok
        
        relations:
          functions:
            via_incoming: partOfBouwblok
            types: [function]
            
            relations:
              components:
                via_incoming: realizes
                types: [component]
                recursive: 5
                require:
                  partOfBouwblok: $.bouwbloks[*].id
```

**Example Output:**

```yaml
entry:
  id: DOC-001
  type: document
  props:
    title: "My Document"
    status: draft
  content: "..."
  
  relations:
    bouwbloks:
      - id: BB-001
        props:
          title: "Bouwblok Core"
        relations:
          functions:
            - id: FUNC-001
              props:
                title: "Publish API"
```

**Files to modify:**

- `internal/views/types_v2.go` (NEW) - New types
- `internal/views/types_v2_test.go` (NEW) - Tests
- `internal/views/loader.go` - Add detection of v2 format

## Security Considerations

- [x] Input sources identified
- [x] Input validation approach defined
- [x] Security-sensitive operations identified
- [x] Error handling doesn't leak sensitive information

No security concerns - pure data structure definition.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined
- [x] Integration test approach defined

**Test Scenarios:**

| Criterion | Test |
|-----------|------|
| Root entry_type/param | `TestQueryNode_RootFields` |
| Nested relations | `TestQueryNode_NestedRelations` |
| Traversal options | `TestQueryNode_TraversalOptions` |
| Require with JSONPath | `TestQueryNode_Require` |
| Output controls | `TestQueryNode_OutputControls` |
| V1 compatibility | `TestFile_ParseV1Compatible` |

**Edge Cases:**

- Empty relations map (leaf node)
- `recursive: 0` vs not set
- `types: []` empty vs not set
- `content: false` vs not set (default true)
- `props: false` vs not set (default true)

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Effort estimated

**Effort:** S (small) - simplified by using explicit `relations:` block

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: simple data structures, no complex logic)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A)

**Design Review Findings:** Skipped - this is straightforward type definitions with no complex logic to review.
