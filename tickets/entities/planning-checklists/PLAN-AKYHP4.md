---
id: PLAN-AKYHP4
type: planning-checklist
title: 'Planning: Schema property labels render on generic entity details'
started: "2026-10-02"
completed: "2026-10-02"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** Add an optional `label` to property schema definitions, return it in
the v1 schema API, and use it for generic entity-detail property names. Preserve
the current generated field label when no schema label exists. This does not
change enum value labels or configured view-field labels.

**Acceptance Criteria:**
1. A schema property with `label: Behandelstrategie` renders that wording on the generic entity detail page while its key remains `behandeling` for data and lookup.
2. A property without `label` continues to render the existing field label.
3. The v1 `_schema` response includes a configured property label; absent labels remain omitted.

## Research

- [x] For larger features: run `/research` to create a structured research doc — N/A: small additive schema/display change
- [x] Searched for existing libraries that solve this problem — N/A: existing schema/display pipeline is the appropriate implementation
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects — N/A: no external library needed
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — small change.

**Existing Solutions:**
- `PropertyDef` already carries enum `labels`; `internal/dataentry/api_v1.go:76` is the single schema serialization point for property definitions.
- `EntityDetail.vue:1429` maps view section fields to `PropertyItem`; `PropertyDisplay.vue` renders `PropertyItem.label`.
- Reuse the property definition passed to `mapFieldsToProperties`; no new lookup mechanism is needed.
- No external library or reference implementation applies. The affected concept is `metamodel-types`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Add optional `Label` to the YAML metamodel `PropertyDef`
and v1 wire `PropertyDef`, copy it in `toV1PropertyDef`, mirror it in the
frontend `PropertyDef`, and resolve the display label as `def?.label ||
field.label` in `mapFieldsToProperties`. This preserves authored view labels as
fallback and leaves property identity unchanged.

**Files to modify:**
- `internal/metamodel/types.go`
- `internal/apiwire/v1/responses.go`
- `internal/dataentry/api_v1.go`
- `frontend/src/types/schema.ts`
- `frontend/src/components/entity/EntityDetail.vue`
- Focused Go and frontend tests for schema serialization and generic detail rendering.

**Alternatives:**
- Derive display text from property keys: rejected because schema labels are explicitly authored and may be localized or differ from a mechanical title case.
- Put label resolution in `PropertyDisplay`: rejected because the owning entity type/property definition is already resolved by `EntityDetail`, and `PropertyDisplay` should remain a generic renderer.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Property labels are operator-authored schema
configuration already available to the server. Treat as display text; no new
user input, identity, or path handling is introduced. Existing schema
parsing/validation handles malformed YAML.

**Security-Sensitive Operations:** None. The label is schema metadata; entity
value visibility and authorization behavior do not change.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
- Criterion 1: mount generic detail with a schema-defined property label and assert the label text.
- Criterion 2: omit schema label and assert existing field-label fallback.
- Criterion 3: exercise `toV1PropertyDef` / `_schema` handler serialization for a present and absent label.

**Edge Cases:**
- Empty or absent label uses existing fallback.
- Unicode and punctuation are rendered as text, not interpreted as HTML.
- Underlying property name and value remain unchanged.

**Negative Tests:** No new failure mode is introduced; malformed schema remains
rejected by existing parsing. Assert absent label omission/fallback.

**Integration:** Frontend component test covers displayed output; Go API-level
test covers schema serialization boundary.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Low risk. Main risk is accidentally changing property identity or
overriding a configured view-field label; keep the key untouched and use schema
label only where a `PropertyDef` exists, with fallback otherwise. Effort: s.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] N/A - Internal change, no user-facing docs needed. This exposes an optional display label through existing schema configuration and needs no new workflow documentation.

## Design Review

- [x] Run `/design-review` before starting implementation — post-hoc ticket; design reviewed against the existing schema-to-UI pipeline
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** None. The change follows the existing `PropertyDef`
serialization path and keeps schema labels display-only.
