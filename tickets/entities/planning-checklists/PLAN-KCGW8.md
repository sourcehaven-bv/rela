---
id: PLAN-KCGW8
type: planning-checklist
title: 'Planning: Add display_property to entity-type metamodel'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

The ticket covers the problem, scope, and 5 acceptance criteria in detail.
Recap: add an optional `display_property` field to `EntityDef`, override
`GetPrimaryProperty` when set, fail metamodel-load if it names a missing
property, **stringify non-string property values in `DisplayTitle`** so an
explicit `display_property: status` (enum) actually renders the value instead of
falling back to the entity ID. No frontend or user-metamodel changes — those are
adoption steps for follow-up tickets.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing code touched:**
- `internal/metamodel/types.go` — `EntityDef` struct. Add the new field
next to `Properties`.
- `internal/metamodel/entity_def.go:59-101` — `GetPrimaryProperty()` and
`DisplayTitle()`. Both need updates: explicit-override on the former,
`fmt.Sprintf("%v", val)` stringification on the latter (RR-9CW5N).
- `internal/metamodel/loader.go:283-310` — `validateEntitySemantics()` —
extend with a `display_property` existence check **and** explicit whitespace
check (RR-HDAX8).
- `docs-project/entities/guides/GUIDE-metamodel.md` — entity-types field
table. Add `display_property` row.
- `docs/metamodel.md` — derived; rebuilt by `just docs`.

**Existing patterns followed:**
- Validation error formatting: `fmt.Sprintf("entity %q: ...", name)`.
- Optional YAML field: `\`yaml:"display_property,omitempty"\``(see`IDPrefix`, `Color`, etc.).
- Test pattern: `entity_def_test.go` already tests `GetPrimaryProperty` +
`DisplayTitle` — extend the same suite.

**Not applicable:**
- Frontend: SPA renders `_title`, server-computed via
`Metamodel.DisplayTitle`, which calls `GetPrimaryProperty` — automatic
propagation, no SPA work.
- MCP: `mcp/server.go` exposes the metamodel via `get_metamodel`; the
new field rides through as one more YAML key (RR-GO9T7).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified

**Technical approach.**

1. **Add the field** to `EntityDef` (types.go):
   ```go
   DisplayProperty string `yaml:"display_property,omitempty"`
   ```

2. **Update `GetPrimaryProperty()`** (entity_def.go) — single-line
prepend before the existing priority-list logic:
   ```go
   if e.DisplayProperty != "" {
       return e.DisplayProperty
   }
   // (existing priority/fallback logic unchanged)
   ```
No requirement that the property exists or is type=string — responsibility moves
to load-time validation. This keeps the runtime path zero-overhead and avoids
returning "" when an author sets a genuinely-empty-but-explicit property name.

3. **Update `DisplayTitle()`** (entity_def.go) to stringify non-string
values (RR-9CW5N resolution). Today the function does `val.(string)` and falls
through to the entity ID on assertion failure. Change to:
   ```go
   if val, ok := properties[primary]; ok {
       if s, ok := val.(string); ok && s != "" {
           return s
       }
       // Stringify non-string values so authors can use enums / numbers
       // / booleans as display fields. fmt.Sprintf("%v", nil) is "<nil>"
       // so guard against that.
       if val != nil {
           if s := fmt.Sprintf("%v", val); s != "" {
               return s
           }
       }
   }
   return id
   ```

4. **Add load-time validation** to `validateEntitySemantics()`. New
block per entity, using accumulating-error pattern (RR-3FC3O confirmed this is
the right layer):
   ```go
   if dp := def.DisplayProperty; dp != "" {
       if dp != strings.TrimSpace(dp) {
           errs = append(errs, fmt.Sprintf(
               "entity %q: display_property %q has leading or trailing whitespace",
               name, dp))
       } else if _, ok := def.Properties[dp]; !ok {
           errs = append(errs, fmt.Sprintf(
               "entity %q: display_property %q is not a defined property (have: %s)",
               name, dp, strings.Join(sortedKeys(def.Properties), ", ")))
       }
   }
   ```
Whitespace check is explicit (RR-HDAX8) so the diagnostic is honest and the
behavior doesn't depend on a side effect in `validateEntityStructure`.

5. **Tests** in `entity_def_test.go` and `loader_test.go`:
   - `GetPrimaryProperty_explicitDisplayProperty` — sets `DisplayProperty`,
asserts the value is returned.
   - `GetPrimaryProperty_displayPropertyOverridesPriority` — sets
`DisplayProperty: "naam"` even when `title` exists; asserts naam wins.
   - `GetPrimaryProperty_emptyDisplayPropertyFallsThrough` — empty
string; asserts existing priority list runs.
   - `DisplayTitle_explicitDisplayProperty` — `DisplayProperty: "naam"`,
entity has `naam: "Foo"`; asserts "Foo" returned.
   - `DisplayTitle_explicitDisplayPropertyMissing` — `DisplayProperty:
"naam"`, entity has no `naam` value; asserts ID returned.
   - **`DisplayTitle_explicitDisplayPropertyEnum`** (RR-9CW5N) —
`DisplayProperty: "status"`, entity has `status: "open"` (string in YAML,
behaves like enum); asserts "open" returned. Plus a numeric/ boolean variant to
pin the stringification.
   - `Load_displayPropertyMissing` — YAML with `display_property:
nonexistent`; asserts SchemaValidationError mentioning entity type + missing
property name.
   - `Load_displayPropertyWhitespace` (RR-HDAX8) — `display_property:
" titel "`; asserts SchemaValidationError mentioning whitespace (distinct from
the missing-property error).
   - `Load_displayPropertySucceeds` — YAML with `display_property:
titel`; asserts no error and the field round-trips.
   - **`Load_displayPropertyYAMLNull`** (RR-HP5IE) — YAML with
`display_property:` (no value, parses as null); asserts no error, field is
empty, `GetPrimaryProperty` falls through.
   - **`Load_displayPropertyCaseSensitive`** (RR-GO9T7) —
`display_property: TITEL` when property is `titel`; asserts load error (lookup
is case-sensitive).
   - **`Load_displayPropertyAcrossIncludes`** (RR-GO9T7) — parent
metamodel includes a child file; child entity has `display_property:
<its-prop>`; asserts validation runs on merged result + succeeds.
   - **`Load_allShippedMetamodels`** (RR-G175B) — globs every
`metamodel.yaml` under the repo (`tickets/`, `docs-project/`, `prototypes/`,
`e2e/` test fixtures) and asserts each loads without error. Catches typos when
display_property gets adopted in a dogfood metamodel.

6. **Documentation.** Insert one row in `GUIDE-metamodel.md`'s entity-
types field table. Describe:
   - explicit override of the priority list,
   - load-time existence + whitespace validation,
   - the runtime stringification behavior for non-string values.
`just docs` rebuilds `docs/metamodel.md`.

**Alternatives considered.**

- *Validate "must be type=string and required"* — rejected. With
RR-9CW5N's stringification fix, enums / numbers / booleans render correctly.
Adding a strict type check would prevent legitimate use cases without solving a
real bug. The existence + whitespace checks alone are what load-time validation
catches.

- *Add at PropertyDef level (`is_display: true`)* — rejected. Each
entity type can have only one display property; a per-property boolean makes it
possible to declare two ("first match wins" or "validation error"? both worse).
A scalar field on the entity type is unambiguous.

- ~~*Templated formats now (`display_format: "{naam} ({status})"`)*~~ —
removed from "future-compat" framing per RR-X6HBS. The single-property design
stands on its own. If templating is ever needed, that's a separate ticket with
its own design decisions (likely a parsed-template field cached on `EntityDef`,
set during `loader.go`'s `parseRaw`). Not promising forward-compatibility we
can't guarantee.

**Files to modify.**

Go:
- `internal/metamodel/types.go` — new field
- `internal/metamodel/entity_def.go` — explicit-override check +
stringification fix
- `internal/metamodel/entity_def_test.go` — new test cases
- `internal/metamodel/loader.go` — validation block
- `internal/metamodel/loader_test.go` — load-time validation tests +
shipped-metamodel guard

Documentation:
- `docs-project/entities/guides/GUIDE-metamodel.md` — entity-types row
- `docs/metamodel.md` — derived (rebuilt by `just docs`)

## Security Considerations

- [x] Input sources identified
- [x] Input validation approach defined
- [x] Security-sensitive operations identified
- [x] Error handling doesn't leak sensitive information

**Input sources.**
- `display_property` value comes from the metamodel YAML, which is
trusted (in-repo, version-controlled). Same trust model as every other metamodel
field.

**Validation.**
- Existence check + whitespace check at load time. Failures surface
via `SchemaValidationError` with the entity type and property name.

**Error handling.**
- Errors include the entity type name and the property name the author
asked for, plus the list of valid property names. No filesystem paths, no stack
traces, no input bytes — just the schema-relative information the author needs
to fix the metamodel.

**Threat model.** None new. The field is a string lookup against an in-process
map; no parser, no expansion, no external calls.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined
- [x] Integration test approach defined

**Test scenarios.** See section 5 of Approach for the full list (12 cases — 6
unit, 6 loader). Mapping to ACs:

- AC1: `Load_displayPropertySucceeds`, `Load_displayPropertyYAMLNull`
- AC2: `GetPrimaryProperty_explicitDisplayProperty`, `_overridesPriority`,
`DisplayTitle_explicitDisplayProperty`, `_explicitDisplayPropertyMissing`,
`_explicitDisplayPropertyEnum`
- AC3: `_emptyDisplayPropertyFallsThrough` + existing `entity_def_test.go`
unchanged
- AC4: `Load_displayPropertyMissing`, `_displayPropertyWhitespace`,
`_displayPropertyCaseSensitive`, `_displayPropertyAcrossIncludes`
- AC5: `just docs` runs cleanly
- (defensive) `Load_allShippedMetamodels` — catches typos in dogfood
metamodels once display_property gets adopted.

**Edge cases.**

- `display_property: ""` (empty string) → behave as if unset. Tested.
- `display_property:` (YAML null) → behaves identically. Tested
(RR-HP5IE).
- `display_property: titel` but property value is non-string (enum,
number, boolean) → stringify via `fmt.Sprintf("%v", val)`, ID fallback for
empty-after-stringification. Tested (RR-9CW5N).
- `display_property: " titel "` (whitespace) → explicit error. Tested
(RR-HDAX8).
- `display_property: TITEL` vs `titel` (case mismatch) → error
(Go map lookup is case-sensitive). Tested (RR-GO9T7).
- `display_property: titel` defined in an included file → validation
runs on merged result. Tested (RR-GO9T7).
- Reserved property name (`id`, `_title`, `mod_time`) → out of scope;
`validateEntityStructure` already rejects reserved names as property
declarations.

**Negative tests.**

- `display_property: nonexistent` → load error.
- `display_property: " titel "` → load error (whitespace).
- `display_property: TITEL` (case-mismatched) → load error.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed
- [x] Effort estimated

**Risks.**

| Risk | Impact | Mitigation |
|------|--------|------------|
| Breaking metamodels that already have a property called `display_property` | Very low — that property name was never reserved; no usage in dogfood/VWS metamodels | `Load_allShippedMetamodels` test catches it |
| Stringification fix changes existing behavior for some entity (e.g. an entity has a non-string `title` field) | Very low — `title`/`name`/`label` are conventionally strings; non-string values were going to fall back to ID anyway | Existing `entity_def_test.go` cases pin the string + empty-string + missing branches; new test pins the non-string path |
| Validation changes break test fixtures with bogus metamodels | Low | Validation only fires when `display_property` is explicitly set |

**Effort:** S (small). ~70 lines of Go now (added stringification + 2 extra test
cases vs. the original estimate of 50). No frontend, no migration, no e2e.

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

**Documentation impact.**
- [x] User guide / reference docs — `docs-project/.../GUIDE-metamodel.md`
entity-types field table.
- [x] ~~CLI help text~~ (N/A: no CLI changes)
- [x] ~~CLAUDE.md~~ (N/A: no architectural pattern change)
- [x] ~~README.md~~ (N/A)
- [x] ~~API docs~~ (N/A: metamodel YAML schema documented in
GUIDE-metamodel.md)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

| RR | Severity | Status | Resolution |
|----|----------|--------|------------|
| RR-9CW5N | critical | addressed | Stringify non-string values in `DisplayTitle` via `fmt.Sprintf("%v", val)` + dedicated test |
| RR-HDAX8 | significant | addressed | Explicit whitespace check in `validateEntitySemantics` with dedicated diagnostic |
| RR-X6HBS | significant | addressed | Dropped `display_format` forward-compat promise; documented as separate-design decision if ever needed |
| RR-HP5IE | minor | addressed | Added `Load_displayPropertyYAMLNull` test |
| RR-G175B | minor | addressed | Added `Load_allShippedMetamodels` test |
| RR-GO9T7 | minor | addressed | Added case-sensitivity + includes-merge tests |
| RR-3FC3O | nit | addressed | Confirmed `validateEntitySemantics` is the right layer (closing for the record) |
| RR-HDCZ6 | nit | addressed | No perf concern; closed for the record |
