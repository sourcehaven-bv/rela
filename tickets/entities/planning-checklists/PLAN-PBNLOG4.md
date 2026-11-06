---
id: PLAN-PBNLOG4
type: planning-checklist
title: 'Planning: Validation rules enforced on write (enforce: write)'
started: "2026-11-06"
completed: "2026-11-06"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: an `enforce: write` key on `validations:` rules. The rule's
`when`/`then` (filter clauses or `condition`/`then_condition` expressions) are
evaluated through `internal/predicate` against the new entity state on every
persisting write (create, update, patch, restore, automation CAS writes, copy,
conversion). A violation refuses the write with 422, in both validation modes.
Out: `related(...)`, `relations:`, `content:` and Lua/AI on enforced rules
(refused at schema load); the create dry run.

**Acceptance Criteria:**
1. A violated enforced rule refuses create, update and patch and leaves the stored row unchanged (TestWriteRule_RefusesWrite).
2. An advisory rule keeps warning only (TestWriteRule_RefusesWrite advisory case).
3. Rules apply per entity_type and face scope (writerule TestCheck).
4. Lua, relations, content, related() and a non-error severity are refused at load (TestValidationEnforce_Load, TestCompile_Refuses).
5. The message names the rule and not the property value (TestCheck_MessageNamesNoValue).

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: reuses the predicate engine and existing rule shape)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: internal rule engine exists)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal feature)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** `predicatefns.Evaluator` (Compile, CompileFilter,
Matches) already evaluates `condition:` and filter clauses;
`metamodel.ValidationRule` already carries when/then.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** New leaf package `internal/writerule` compiles enforced
rules once in `entitymanager.New` and checks an entity in-process.
`writeWithUniqueCheck` calls it before every persist; copy and conversion call
it on their targets. Violations become a `ValidationErrorRule` in the existing
hard partition. Rejected: running the full validator (Lua, relations) on the
write path, because it would run user Lua and per-row graph reads on writes;
allowing `related()`, because it needs gated graph reads inside the write.

**Files to modify:** internal/metamodel/{types,loader,validation}.go,
internal/writerule/,
internal/entitymanager/{manager,validation,unique,copy,conversion_flow}.go,
internal/configedit/allowlist.go, .go-arch-lint.yml,
docs-project/entities/guides/GUIDE-metamodel.md.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Rules come from operator-authored schema.yaml;
the key is checked against an allowlist of values and is locked in Configure.
Entity properties come from the write request.

**Security-Sensitive Operations:** No Lua or AI on the path (CLAUDE.md,
checkers/predicate rules). The 422 message names the rule and its description
only, not property values.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** See acceptance criteria; unit tests in writerule,
integration tests through a real Manager.

**Edge Cases:** Rule without entity_type; face-scoped rule; when not matched;
then-expression evaluation error.

**Negative Tests:** Bad expressions and forbidden keys fail schema load;
violated rules refuse writes.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** A create stores the row before on-create automation; a rule that only
automation can satisfy refuses the first write. Documented. Effort: m.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/metamodel.md - "Enforcing a rule on write (enforce: write)"

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: small extension of the existing rule shape and predicate engine; the code review covers it)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review run)

**Design Review Findings:** N/A
