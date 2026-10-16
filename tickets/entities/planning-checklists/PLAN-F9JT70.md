---
id: PLAN-F9JT70
type: planning-checklist
title: 'Planning: Traverse condition: predicate expressions on view traverse steps'
started: "2026-10-15"
completed: "2026-10-15"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**
In: a `condition:` key on view traverse steps (relation steps and
`mentioned_by`), compiled at load, evaluated on the redacted entity; a load
error for an unparseable traverse `where:` (BUG-WHEREWIDE).
Out: `condition:` in side panels (refused at load); `related(...)` in a
traverse condition (refused); any change to `where:` syntax.

**Acceptance Criteria:**
1. `entity.type == 'taak' and entity.status == 'gereed'` keeps only finished tasks.
2. `entity.type == 'project' or (entity.type == 'taak' and entity.status ~= 'gereed')` keeps unfinished tasks and every project.
3. A property that only another collected type declares reads as nil, not a compile error.
4. Existing `where:` expressions behave as before; `condition:` is ANDed with them.
5. A condition that does not compile, and an unparseable `where:`, fail `rela validate` and startup with the view, step and type named.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: follows the list `condition:` design)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: in-repo predicate engine)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- List `condition:` (`dataentryconfig.List.Condition`, `conditionlint.CompileViewConditions`, `dataentry.applyViewCondition`): the key, compile-at-load and redacted evaluation are reused.
- Next-action conditions compile once per entity type and must compile for all of them (`conditionlint.compileNextActionSource`); the same rule applies per collected type.
- Action `when:` keys one program per type (`ActionConditionID`); `TraverseConditionID` follows it.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
`ViewTraverse.Condition` is compiled by `CompileViewConditions` per type from
`TraverseTargetTypes`, with `Evaluator.CompileForCollection` (entity record
plus `id`, `type`, and the other collected types' properties). The view engine
resolves the lookup once per execution through the existing
`ViewConditionFunc` seam and filters each rule's result per type after
`where:`.

Alternatives rejected: parsing `where:` as a predicate and falling back to the
filter DSL (dialect sniffing, which `.claude/rules/predicate.md` forbids, and
`status = gereed` is not valid Lua anyway); a `[]string` of ANDed filter
clauses like calendars (no `or`, no grouping); adding `id`/`type` to the
shared `EntityRecordType` (would reach queryplan's SQL lowering, where they
are not properties).

**Files to modify:**
`internal/dataentryconfig/{config.go,traverse_condition.go,validate.go,validate_mentions.go}`,
`internal/predicatefns/evaluator.go`, `internal/conditionlint/viewcondition.go`,
`internal/dataentry/{views.go,views_mentions.go,views_handler.go,app.go,views_traverse_condition.go}`,
`docs-project/entities/guides/GUIDE-data-entry.md`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
The expression comes from `data-entry.yaml` only, never from a request. It is
compiled at load by the sandboxed predicate engine.

**Security-Sensitive Operations:**
Membership decided on field values: the condition reads the redacted entity,
so a hidden field reads as nil and cannot decide membership. Collections are
still row-gated on the way out.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
1-4: `TestExecuteViewWithCondition` (mixed-type traversal) and `TestEvaluator_CompileForCollection`.
5: `TestCompileViewConditions_Traverse`, `TestValidateConfig_TraverseWhereMustParse`, manual `rela validate`.

**Edge Cases:**
Type without the property (nil), conflicting property types across collected
types (own declaration wins), `mentioned_by` (every type), entity of a type the
relation does not declare (dropped).

**Negative Tests:**
Typo in a property, filter-dialect spelling, `related(...)`, no compiler wired,
uncompiled condition at runtime, side panel `condition:`.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
A config with an unparseable traverse `where:` that loads today is refused
after this change. That is the decided fix for BUG-WHEREWIDE (RULING 17).
Effort: m.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/data-entry.md - traverse `condition:` and the `where:` load error

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: reuses the reviewed list `condition:` design)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A)

**Design Review Findings:** none
