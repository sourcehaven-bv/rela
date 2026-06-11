<!-- @managed: claude-workflow v1 -->
---
id: IMPL-IHC7D
type: implementation-checklist
title: 'Implementation: View wire-shape — typed _props + _fields per cards/list row entity'
status: done
---

## Development

- [x] Unit tests written for new code — 9 new tests in `sections_ihc7d_test.go` covering `copyVisibleProperties`, `buildSectionEntityData`, `sectionEntityToV1`, key-set invariant, and the nil/empty wire-shape semantics
- [x] ~~Integration tests written~~ (N/A: change is wire-shape only, no new behavioural flows; the helper + converter tests cover the contract)
- [x] Happy path implemented — `V1ViewEntity._props` + `._fields`; `SectionEntityData.Props` + `.FieldVerdicts`; shared `buildSectionEntityData` helper; `copyVisibleProperties` filtered through `hiddenProperties`; wire converter helper `sectionEntityToV1` called from both `V1ViewEntity` construction sites in `api_v1.go`
- [x] Edge cases from planning handled — hidden-stripped `_props` (RR-FD1A); key-set invariant pinned in Go doc + test (RR-FD1B); both `properties`/`list` AND `content`/`cards` branches wired (RR-FD1C); `serializeRelatedEntityForWire` comment left intact (RR-FD1D); precomputed verdict in `buildSections` (RR-FD1E reverse of alt-b); third converter site at `GroupData.Entities` covered (RR-FD2A)
- [x] Error handling in place — N/A (no new error paths; defensive nil checks on `FieldVerdicts` in `sectionEntityToV1` and `eDef` in `buildSectionEntityData`)

## Test Quality

- [x] Using fixture builders or factories for test data — reuses `verdictBuilder`, `appWithResolver`, and `testViewApp` from the existing test harness
- [x] No hardcoded values in assertions when object is in scope — verdicts compared structurally; key-set invariant test reads back from the same `hiddenProperties` source
- [x] Only specifying values that matter for the test — hidden-stripped test sets only the property under test; key-set invariant test sets both hidden + readonly to verify both axes
- [x] Interpolated values constructed from objects, not hardcoded — N/A (no string interpolation in assertions)
- [x] Property comparisons use original object, not hardcoded strings — `reflect.DeepEqual(sed.Props, ...)` compared against the literal map passed into the entity

## Manual Verification

- [x] Feature manually tested end-to-end — no UI consumer yet (TKT-IHC7C is the consumer). Smoke via `go test ./internal/dataentry/...` and verifying the new fields appear on a view response shape via test introspection
- [x] Each acceptance criterion verified with test scenario from planning — see PLAN-IHC7D ACs 1-9; tests map to ACs 1, 3, 4, 5, 8 directly; ACs 6, 7 (TS types + docs) verified by `npm run typecheck` + diff review; AC 9 (frontend regression) verified by `npm run test:run` (961/961)
- [x] Edge cases manually verified — group-card path dormant but wired (would crash silently if a producer ever populated `GroupData.Entities`); table rows still skip `_props`/`_fields` (correct)

**Verification Evidence:**

- Local `go test ./internal/dataentry/...`: clean (existing + 9 new tests pass)
- Local `go test ./...`: full sweep clean
- Local `just arch-lint`: OK (no warnings)
- Local `npm run typecheck`: clean
- Local `npm run test:run`: 961/961 (no regressions)

## Quality

- [x] Code follows project patterns — `V1ViewEntity._fields` mirrors `V1Entity.FieldAffordances` pointer-to-map idiom; `copyVisibleProperties` mirrors `stripHiddenProperties` style; shared helper extracted as the second call site appeared (TKT-IHC7C will benefit from the same shape)
- [x] ~~Checked for DRY opportunities~~ — extracted `buildSectionEntityData` from the duplicate `properties`/`list` and `content`/`cards` branches in `buildSections` (RR-FD1C); extracted `sectionEntityToV1` from the duplicate V1ViewEntity construction at api_v1.go's top-level entities and group-entities sites (RR-FD2A)
- [x] No security issues introduced — hidden properties are filtered before reaching the wire (RR-FD1A); `_fields` consistency with `V1Entity._fields` ensures the same ACL verdict applies regardless of which surface the consumer reads from
- [x] No silent failures — the wire converter's `nil` checks gate emission (FieldAffordances pointer-to-map idiom); `sectionEntityToV1` falls back cleanly when `FieldVerdicts` is nil (entry-source and table paths)
- [x] No debug code left behind — reviewed diff
