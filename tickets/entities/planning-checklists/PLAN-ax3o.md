---
id: PLAN-ax3o
status: done
title: 'Planning: Add test fixture builders with randomized data'
type: planning-checklist
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**
- IN: Entity builder with fluent API (`Entity()`, `EntityFor()`)
- IN: Relation builder with fluent API
- IN: Random value generation for all property types (string, enum, integer, date, boolean)
- IN: List property support via `WithList()`
- IN: Metamodel-aware auto-fill for required properties
- OUT: Graph builder (can be added later if needed)
- OUT: Refactoring existing tests (separate follow-up ticket)

**Acceptance Criteria:**
1. `testutil.Entity("type").Build()` creates entity with random ID and empty properties
2. `testutil.Entity("type").With("key", "value").Build()` sets property correctly
3. `testutil.Entity("type").WithList("tags", "a", "b").Build()` stores as `[]string`
4. `testutil.EntityFor(meta, "type").Build()` auto-fills all required properties with valid random values
5. `testutil.EntityFor(meta, "type").With("status", "x").Build()` overrides auto-generated value
6. Enum properties get random value from allowed values list
7. `testutil.Relation("type").From("A").To("B").Build()` creates valid relation

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**
- Go's `testing/quick` package has `quick.Value()` for random generation - could use for inspiration
- No external test fixture libraries needed - simple builder pattern suffices
- Existing helpers in codebase:
  - `internal/testutil/testutil.go` - file/dir helpers, assertions (will add builders here)
  - `internal/graph/graph_test.go:newEntity()` - minimal wrapper, package-local
  - `internal/dataentry/app_test.go:testMeta()` - creates test metamodel
- Pattern: fluent builders are common in Go testing (e.g., testify's mock.On().Return())

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. Create `internal/testutil/entity_builder.go`:
   - `EntityBuilder` struct with fluent methods
   - `Entity(entityType string) *EntityBuilder` - simple builder
   - `EntityFor(meta *metamodel.Metamodel, entityType string) *EntityBuilder` - metamodel-aware
   - `With(key string, value interface{})` - set single property
   - `WithList(key string, values ...string)` - set list property
   - `Without(key string)` - skip auto-generation for this key
   - `ID(id string)` - set explicit ID
   - `Content(content string)` - set markdown content
   - `Build() *model.Entity` - finalize

2. Create `internal/testutil/relation_builder.go`:
   - `RelationBuilder` struct
   - `Relation(relType string) *RelationBuilder`
   - `From(id string)`, `To(id string)`, `Build() *model.Relation`

3. Create `internal/testutil/random.go`:
   - `RandomString()` - generates "word-xxxx" style strings
   - `RandomInt(min, max int)` - random integer in range
   - `RandomDate()` - random date within last year
   - `RandomBool()` - random boolean
   - `RandomEnumValue(values []string)` - picks from list
   - `RandomID(prefix string)` - generates "PREFIX-xxxx" style IDs

**Alternatives Rejected:**
- Functional options pattern: More verbose than fluent builder for this use case
- Separate package: Would require import, keeping in testutil is simpler
- Code generation: Overkill for this scope

**Dependencies:**
- `internal/model` - Entity, Relation types
- `internal/metamodel` - Metamodel, EntityDef, PropertyDef types
- `math/rand` - random generation (will use deterministic seed option for reproducible tests)

**Files to modify:**
- `internal/testutil/entity_builder.go` (new)
- `internal/testutil/relation_builder.go` (new)
- `internal/testutil/random.go` (new)
- `internal/testutil/entity_builder_test.go` (new)
- `internal/testutil/relation_builder_test.go` (new)
- `internal/testutil/random_test.go` (new)

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- No external input - test utilities only operate on in-memory data
- Metamodel comes from test setup, not external sources

**Security-Sensitive Operations:**
- None - this is test-only code, not used in production

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
1. AC1: `TestEntityBuilder_Build_RandomID` - verify ID is non-empty and has expected format
2. AC2: `TestEntityBuilder_With_SetsProperty` - verify property is set correctly
3. AC3: `TestEntityBuilder_WithList_StoresAsSlice` - verify []string storage
4. AC4: `TestEntityBuilderFor_AutoFillsRequired` - create test metamodel, verify all required props filled
5. AC5: `TestEntityBuilderFor_WithOverridesAutoFill` - verify explicit value wins
6. AC6: `TestEntityBuilderFor_EnumGetsValidValue` - verify value is in allowed list
7. AC7: `TestRelationBuilder_Build` - verify From, Type, To are set

**Edge Cases:**
- Empty entity type → still works, creates entity with empty Type
- Nil metamodel to EntityFor → panic with clear message
- Unknown entity type in metamodel → panic with clear message
- Property with no values (non-enum custom type) → treated as string
- List property via With() instead of WithList() → should work (coerce to slice)

**Negative Tests:**
- `TestEntityBuilderFor_PanicsOnNilMetamodel`
- `TestEntityBuilderFor_PanicsOnUnknownType`

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Risk: Random values make tests flaky → Mitigation: Use deterministic seed, document that random values shouldn't affect test outcomes
- Risk: Import cycle with metamodel → Mitigation: Check imports, may need interface if cycle occurs
- Risk: Divergence if metamodel changes → Mitigation: Tests will fail fast if property types change

**Effort:** m (medium) - straightforward implementation, well-defined scope

## Design Review

- [ ] Run `/design-review` before starting implementation
- [ ] All critical/significant findings addressed in plan

**Design Review Findings:** <!-- List review-response IDs, e.g., RR-xxxx -->
