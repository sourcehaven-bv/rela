---
id: PLAN-RLYAUT
type: planning-checklist
title: 'Planning: Configurable backlinks section with per-type columns'
started: "2026-10-08"
completed: "2026-10-08"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope:
- A view traverse step that collects the entry's backlinks: `mentioned_by: true` with `from: entry`.
- Per-type definitions on flat sections: `fields_by_type` (list, cards, content, properties) and `columns_by_type` (table). They work for any collection, so they also cover a relation with several target types.
- The built-in "Mentioned by" panel steps aside when the type's view collects backlinks, and `entity_views.<type>.mentioned_by_panel: false` turns it off.

Out of scope:
- Outgoing mentions as a traversal.
- A mentions step from a collection other than `entry` (it would need one index read per row).
- Per-type definitions on `nested` (it already has `parent_columns`/`child_columns`).

**Acceptance Criteria:**
1. A view with `{from: entry, mentioned_by: true, collect_as: mentions}` fills `mentions` with the entities whose body mentions the entry. Hidden sources are absent. The order is the section's `sort:`, or the id order otherwise.
2. The collection holds at most 200 sources. If more exist, or the scan stopped, the section's response says `truncated: true` and the SPA shows a note.
3. `where:` on the step filters the collected sources.
4. `columns_by_type: {ticket: [...], decision: [...]}` on a table renders one group per type, each with its own header and columns. A type not in the map uses `columns:`.
5. `fields_by_type` on list/cards renders each row with its own type's fields. A type not in the map uses `fields:`.
6. Load errors: `mentioned_by` together with `follow`/`follow_incoming`/`recursive`; `mentioned_by` with a `from` other than `entry`; a `*_by_type` key that is not a declared type, or one the collection can never hold; a property the keyed type does not declare; `columns_by_type` together with `group_by`; `columns_by_type` on a non-table section; `fields_by_type` on a table or nested section.
7. The entity page hides `MentionedByPanel` when the type's view has a `mentioned_by` step, or when `entity_views.<type>.mentioned_by_panel` is false.
8. The query budget for a view with a mentions section stays the same at 10 and 50 sources.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A: the design extends existing view config; the prior art is
in the codebase.

**Existing Solutions:**
- Per-type columns: `nested` sections' `parent_columns`/`child_columns` (`internal/dataentryconfig/config.go:1948`, validated by `validateLevelColumns` in `validate.go:1969`). The new keys follow their shape and their rules: a key must be a type the level can hold, and a property must be declared on that type.
- Per-type maps elsewhere: `create.types`, `gantts.<id>.sources`.
- Flat sections today take one list, look values up by property name and leave undeclared ones blank (`sections.go:115`, `sections_nested.go:348`). Validation skips property checks when the collection is mixed (`determineTargetType`, `validate.go:2147`).
- Backlink reads: `visibility.Resolver.MentionedBy` (gated, keyset paged) from TKT-Q6AKUN.
- Libraries: none apply (internal config and wiring).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Config (`internal/dataentryconfig`):
- `ViewTraverse.MentionedBy bool` (`mentioned_by`).
- `ViewSection.FieldsByType map[string][]ViewSectionField` (`fields_by_type`) and `ColumnsByType map[string][]ListColumn` (`columns_by_type`).
- `EntityViewConfig.MentionedByPanel *bool` (`mentioned_by_panel`, nil means shown).
- `validateViews` gains the checks in AC 6. The types a mentions collection can hold are all declared types.

Traversal (`internal/dataentry/views.go`):
- `applyViewTraverse` branches on `rule.MentionedBy`. It calls `viewReader.Resolver().MentionedBy(ctx, entry, "", 200)` once. The result carries the ids at their served faces and the `Truncated` flag (`page.Next != ""` or `page.Truncated`). The ids then go through the same batched `loadViewEntities` and `where` filter as a relation step.
- `viewResult` records truncated collections. `SectionData.Truncated` is set for a flat section whose source was truncated.

Sections (`internal/dataentry/sections.go`):
- `fieldsFor(sec, type)` and `columnsFor(sec, type)` pick the per-type list, or fall back to the section's list.
- The list/cards/content/properties arms call `fieldsFor`.
- The table arm with `columns_by_type` groups rows by type in first-seen order after sorting. Each `GroupData` gets `Columns` and the type's label as its name. Relation columns resolve once per (column, row type), as now.
- Wire: `ViewSection` gains `truncated`, and table groups gain `columns`.

SPA:
- The table group renders its own columns when present.
- A truncated note is shown under a flat section.
- `EntityDetail.vue` hides `MentionedByPanel` per AC 7, deciding from the raw config the SPA already loads.

Alternatives rejected:
- A `columns:` key that takes either a list or a type map: one key with two shapes is harder to read and validate.
- A single `display: mentioned_by` section: it would duplicate every display mode and still not answer the mixed-type question for relations.
- Hiding the panel only by an explicit flag: a view that places backlinks would then show them twice by default.

**Files to modify:**
- internal/dataentryconfig/config.go, validate.go, tests
- internal/dataentry/views.go, sections.go, viewworld.go (as needed), views_handler.go, tests; querybudget_test.go
- internal/apiwire/v1/responses.go; openapi if views are described there
- frontend/src/api/views.ts, components/entity/EntityDetail.vue, tests
- docs/data-entry.md (Traverse Rules, Sections, Entity Views)
- prototypes/data-entry/project/data-entry.yaml: an example

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- `data-entry.yaml` (operator config, not secret): validated at load; unknown types and properties are refused.
- The entry id comes from the URL and is read-gated by the existing view handler before any traversal.

**Security-Sensitive Operations:**
- Source rows come from `Resolver.MentionedBy` on the view reader, so each passes the row gate and is served at a face the world admits. Properties go through the same redaction as every other view row.
- `truncated` is computed after gating, as in TKT-Q6AKUN (the accepted budget signal), so it does not add an existence oracle.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
- AC 1–3: handler tests on `/api/v1/_views/{type}/{id}` with seeded mentions, a hidden source under ACL, `where:`, and 201 sources for truncation.
- AC 4–5: section tests with mixed types; the table groups by type with per-group columns; fallback to `columns:`/`fields:`.
- AC 6: table-driven validation tests, one case per load error.
- AC 7: EntityDetail component tests for the three panel states.
- AC 8: a `storetest.Counting` budget test at 10 and 50 sources.
- e2e or manual check on the demo project.

**Edge Cases:**
- No sources: `empty_message` shows; no panel.
- The entry mentions itself: excluded (the resolver skips it).
- A faced source: shown at its served face.
- A type in `*_by_type` with an empty list: shows title only.
- `sort:` with `columns_by_type`: groups appear in first-seen order of the sorted rows.

**Negative Tests:**
- Each AC 6 config error fails the load with a message naming the view, section and key.
- A backend without a mention index: the collection is empty, not an error (`ErrMentionsUnsupported`).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- A large backlink set makes the view slow: capped at 200, one index read plus one batch load.
- Grouping by type changes table output only when `columns_by_type` is set, so existing views are unchanged.
- The panel rule hides backlinks on a type whose view collects them but never shows the collection. That is a config error the operator can see, and it is documented.

Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/data-entry.md: traverse `mentioned_by`, `fields_by_type`, `columns_by_type`, `entity_views.<type>.mentioned_by_panel`

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** 18 findings (8 significant, 8 minor, 2 nit),
RR-ZQB4ET through RR-TFFNVU. All are addressed in the design and implementation;
each RR records its resolution.
