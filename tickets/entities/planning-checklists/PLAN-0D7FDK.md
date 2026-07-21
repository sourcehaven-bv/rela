---
id: PLAN-0D7FDK
type: planning-checklist
title: 'Planning: Metamodel doc-fields: top-level description, per-enum-value descriptions, transition help (rela-docs phase 1a)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (in/out below)
- [x] Acceptance criteria documented with test scenarios

**Scope**

IN — three additive, optional, backward-compatible doc-fields + example
population:
1. `Metamodel.Description string` (`yaml:"description,omitempty"`, top-level). **Must also add `"description"` to `validTopLevelKeys` in loader.go — the parser rejects unknown top-level keys, so without this a root `description:` fails to load.**
2. Per-enum-value descriptions on `CustomType` — `Descriptions map[string]string` (`yaml:"descriptions,omitempty"`), keyed by value, mirroring the existing `Labels map[string]string`. Nested → no allowlist change needed.
3. `TransitionDef.Help string` (`yaml:"help,omitempty"`). Nested → no allowlist change.
4. Populate the fields in an in-tree example project.

OUT — the `rela docs` generator (phase 2); ACL RoleDef.Description (phase 1b); a
`docs:` structured block (research rejected A2); any enforcement/validation
behavior change (display-only fields).

**Acceptance Criteria**
1. Root `description:` parses into `Metamodel.Description`, readable; absent → "". Test: parse fixture with/without.
2. CustomType `descriptions:` parses into `CustomType.Descriptions` keyed by value, readable; absent → nil. Test: parse fixture.
3. TransitionDef `help:` parses into `TransitionDef.Help`, readable; absent → "". Test: parse fixture.
4. Backward-compatible: an existing metamodel with none of the fields loads + validates unchanged; parse→marshal round-trip preserves all three. Test: round-trip + the existing metamodel test corpus still passes.
5. `description:` is accepted as a valid top-level key (not flagged by checkUnknownKeys). Test: a metamodel with root `description:` loads without a SchemaValidationError.
6. At least one in-tree example populates all three (see Approach for which).
7. Tests cover parse + absence + round-trip for each field.

## Research

- [x] ~~/research~~ (done: RES-EK7LSA covers the whole arc)
- [x] Searched codebase for the field-add pattern
- [x] Reviewed prior art

**Existing Solutions / prior art**
- Field shape mirrors `EntityDef.Description`, `CustomType.Labels`, `CustomType.Description`, `TransitionDef.Label` — all plain struct tags in `internal/metamodel/types.go`.
- Parse path: `metamodel.Parse` → `parseRaw` → `yaml.Unmarshal` (struct tags) + `checkUnknownKeys` (loader.go:746, TOP-LEVEL keys only) + `extractPropertyOrder` (yaml.Node pass for entity property order only — irrelevant to these fields).
- **Key gotcha:** `validTopLevelKeys` (loader.go:16) allowlists top-level keys; `Metamodel.Description` needs `"description"` added there. Nested fields (CustomType.Descriptions, TransitionDef.Help) are NOT allowlisted — standard unmarshal handles them.

## Approach

- Add `Description string` to `Metamodel` struct (types.go:16) + `"description": true` to `validTopLevelKeys` (loader.go:16).
- Add `Descriptions map[string]string` to `CustomType` (types.go:131), tag `descriptions,omitempty`, doc-comment distinguishing it from `Labels` (display text) — `Descriptions` = the *meaning*/prose of a value.
- Add `Help string` to `TransitionDef` (types.go:158), tag `help,omitempty`.
- No accessor methods strictly required (fields are exported); add small doc comments.
- **Example population:** `tickets/`, `docs-project/`, `prototypes/*` metamodels have status enums but NONE declare `transitions:` today. So: (a) add root `description:` + per-value `descriptions:` to an existing example that has a status enum (e.g. `prototypes/data-entry/project/metamodel.yaml` — the showcase project), and (b) to exercise transition `help`, add a small state machine (transitions + help) to that same prototype metamodel. Prototypes is the right home for showcase schemas; keep it realistic. Validate the project still passes `rela validate` after.

**Files to modify:**
- `internal/metamodel/types.go` (3 fields)
- `internal/metamodel/loader.go` (validTopLevelKeys)
- `internal/metamodel/*_test.go` (parse/absence/round-trip tests)
- one `prototypes/**/metamodel.yaml` (example population)
- possibly `docs/metamodel.md` source (`docs-project/entities/guides/GUIDE-metamodel.md`) documenting the new fields — likely folded into phase 2, but a short note here is cheap.

## Security Considerations

- [x] Input sources identified: metamodel YAML (operator-authored, trusted config — same trust level as all other metamodel fields).
- [x] Validation approach: none needed — free-form prose strings, display-only. No injection surface (they're rendered as Markdown by the future generator, which escapes/handles that at render time, not here).
- [x] No security-sensitive operations. ACL/enforcement untouched.
- [x] Error handling: a malformed `descriptions:` (e.g. non-map) yields a standard yaml unmarshal error — same as any mistyped field.

## Test Plan

- [x] Scenarios per AC (above)
- [x] Edge cases identified
- [x] Negative cases defined
- [x] Integration approach: the existing metamodel test corpus + `rela validate` on the populated example project.

**Edge Cases:**
- All three absent → zero values, no behavior change (the backward-compat case).
- `descriptions:` with a key not in `values:` → tolerated (display-only; not validated — matches how `Labels` behaves; confirm Labels isn't strictly validated and mirror it).
- Empty string values → preserved as-is.
- Round-trip (parse→marshal→parse) preserves all three.

**Negative Tests:**
- Root `description:` must NOT be rejected by checkUnknownKeys (AC5) — regression guard for the allowlist change.

## Risk Assessment

- [x] Technical risks assessed
- [x] Security risks assessed (none)
- [x] Effort estimated: s

**Risks:**
- Forgetting the `validTopLevelKeys` entry → root `description:` silently rejected. Mitigated by AC5 + its explicit test.
- `Descriptions` vs `Labels` confusion for authors → clear doc comments + the example project showing both used together.
- Marshal path: confirm `yaml.Marshal` round-trips the new fields (it will, given struct tags) — covered by the round-trip test.

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist created on entering implementation

**Documentation Impact:**
- [x] docs/metamodel.md (via source guide) — document the three new fields (Custom Types section for descriptions/help; a top-level `description:` note). Small addition; could also ride phase 2, but cheap to do here.
- [x] N/A others

## Design Review

- [x] ~~/design-review~~ — approach is a mechanical additive-field change following an established pattern; design settled in research RES-EK7LSA and this plan. No novel design surface.
- [x] No open critical/significant findings.

**Design Review Findings:** N/A — additive fields mirroring existing ones; the
one non-obvious point (validTopLevelKeys) is captured with a dedicated AC +
test.
